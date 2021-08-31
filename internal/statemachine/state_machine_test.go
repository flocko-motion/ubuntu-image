package statemachine

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/canonical/ubuntu-image/internal/commands"
	"github.com/canonical/ubuntu-image/internal/helper"
	"github.com/snapcore/snapd/gadget"
	"github.com/snapcore/snapd/gadget/quantity"
	"github.com/snapcore/snapd/osutil"
)

var testDir = "ubuntu-image-0615c8dd-d3af-4074-bfcb-c3d3c8392b06"

// for tests where we don't want to run actual states
var testStates = []stateFunc{
	{"test_succeed", func(*StateMachine) error { return nil }},
}

// for tests where we want to run all the states
var allTestStates = []stateFunc{
	{"make_temporary_directories", (*StateMachine).makeTemporaryDirectories},
	{"prepare_gadget_tree", func(statemachine *StateMachine) error { return nil }},
	{"prepare_image", func(statemachine *StateMachine) error { return nil }},
	{"load_gadget_yaml", func(statemachine *StateMachine) error { return nil }},
	{"populate_rootfs_contents", func(statemachine *StateMachine) error { return nil }},
	{"populate_rootfs_contents_hooks", func(statemachine *StateMachine) error { return nil }},
	{"generate_disk_info", func(statemachine *StateMachine) error { return nil }},
	{"calculate_rootfs_size", func(statemachine *StateMachine) error { return nil }},
	{"prepopulate_bootfs_contents", func(statemachine *StateMachine) error { return nil }},
	{"populate_bootfs_contents", func(statemachine *StateMachine) error { return nil }},
	{"populate_prepare_partitions", func(statemachine *StateMachine) error { return nil }},
	{"make_disk", func(statemachine *StateMachine) error { return nil }},
	{"generate_manifest", func(statemachine *StateMachine) error { return nil }},
	{"finish", (*StateMachine).finish},
}

// define some mocked versions of go package functions
func mockCopyBlob([]string) error {
	return fmt.Errorf("Test Error")
}
func mockLayoutVolume(string, string, *gadget.Volume, gadget.LayoutConstraints) (*gadget.LaidOutVolume, error) {
	return nil, fmt.Errorf("Test Error")
}
func mockNewMountedFilesystemWriter(*gadget.LaidOutStructure,
	gadget.ContentObserver) (*gadget.MountedFilesystemWriter, error) {
	return nil, fmt.Errorf("Test Error")
}
func mockMkfsWithContent(typ, img, label, contentRootDir string, deviceSize, sectorSize quantity.Size) error {
	return fmt.Errorf("Test Error")
}
func mockReadDir(string) ([]os.FileInfo, error) {
	return []os.FileInfo{}, fmt.Errorf("Test Error")
}
func mockReadFile(string) ([]byte, error) {
	return []byte{}, fmt.Errorf("Test Error")
}
func mockWriteFile(string, []byte, os.FileMode) error {
	return fmt.Errorf("Test Error")
}
func mockMkdir(string, os.FileMode) error {
	return fmt.Errorf("Test error")
}
func mockMkdirAll(string, os.FileMode) error {
	return fmt.Errorf("Test error")
}
func mockOpenFile(string, int, os.FileMode) (*os.File, error) {
	return nil, fmt.Errorf("Test error")
}
func mockRemoveAll(string) error {
	return fmt.Errorf("Test error")
}
func mockCreate(string) (*os.File, error) {
	return nil, fmt.Errorf("Test error")
}
func mockCopyFile(string, string, osutil.CopyFlag) error {
	return fmt.Errorf("Test error")
}
func mockCopySpecialFile(string, string) error {
	return fmt.Errorf("Test error")
}

// Fake exec command helper
var testCaseName string

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestExecHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	tc := "TEST_CASE=" + testCaseName
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1", tc}
	return cmd
}

// This is a helper that mocks out any exec calls performed in this package
func TestExecHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)
	args := os.Args

	// We need to get rid of the trailing 'mock' call of our test binary, so
	// that args has the actual command arguments. We can then check their
	// correctness etc.
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}

	// I think the best idea I saw from people is to switch this on test case
	// instead on the actual arguments. And this makes sense to me
	switch os.Getenv("TEST_CASE") {
	case "TestGeneratePackageManifest":
		fmt.Fprint(os.Stdout, "foo 1.2\nbar 1.4-1ubuntu4.1\nlibbaz 0.1.3ubuntu2\n")
		break
	case "TestFailedSetupLiveBuildCommands":
		// throwing an error here simulates the "command" having an error
		os.Exit(1)
		break
	}
}

// testStateMachine implements Setup, Run, and Teardown() for testing purposes
type testStateMachine struct {
	StateMachine
}

// testStateMachine needs its own setup
func (TestStateMachine *testStateMachine) Setup() error {
	// set the states that will be used for this image type
	TestStateMachine.states = allTestStates

	// do the validation common to all image types
	if err := TestStateMachine.validateInput(); err != nil {
		return err
	}

	// if --resume was passed, figure out where to start
	if err := TestStateMachine.readMetadata(); err != nil {
		return err
	}

	return nil
}

// TestUntilThru tests --until and --thru with each state
func TestUntilThru(t *testing.T) {
	testCases := []struct {
		name string
	}{
		{"until"},
		{"thru"},
	}
	for _, tc := range testCases {
		t.Run("test "+tc.name, func(t *testing.T) {
			for _, state := range allTestStates {
				// run a partial state machine
				var partialStateMachine testStateMachine
				partialStateMachine.commonFlags, partialStateMachine.stateMachineFlags = helper.InitCommonOpts()
				tempDir := filepath.Join("/tmp", "ubuntu-image-"+tc.name)
				if err := os.Mkdir(tempDir, 0755); err != nil {
					t.Errorf("Could not create workdir: %s\n", err.Error())
				}
				defer os.RemoveAll(tempDir)
				partialStateMachine.stateMachineFlags.WorkDir = tempDir

				if tc.name == "until" {
					partialStateMachine.stateMachineFlags.Until = state.name
				} else {
					partialStateMachine.stateMachineFlags.Thru = state.name
				}

				if err := partialStateMachine.Setup(); err != nil {
					t.Errorf("Failed to set up partial state machine: %s", err.Error())
				}
				if err := partialStateMachine.Run(); err != nil {
					t.Errorf("Failed to run partial state machine: %s", err.Error())
				}
				if err := partialStateMachine.Teardown(); err != nil {
					t.Errorf("Failed to teardown partial state machine: %s", err.Error())
				}

				// now resume
				var resumeStateMachine testStateMachine
				resumeStateMachine.commonFlags, resumeStateMachine.stateMachineFlags = helper.InitCommonOpts()
				resumeStateMachine.stateMachineFlags.Resume = true
				resumeStateMachine.stateMachineFlags.WorkDir = partialStateMachine.stateMachineFlags.WorkDir

				if err := resumeStateMachine.Setup(); err != nil {
					t.Errorf("Failed to resume state machine: %s", err.Error())
				}
				if err := resumeStateMachine.Run(); err != nil {
					t.Errorf("Failed to resume state machine from state: %s\n", state.name)
				}
				if err := resumeStateMachine.Teardown(); err != nil {
					t.Errorf("Failed to resume state machine: %s", err.Error())
				}
				os.RemoveAll(tempDir)
			}
		})
	}
}

// TestInvalidStateMachineArgs tests that invalid state machine command line arguments result in a failure
func TestInvalidStateMachineArgs(t *testing.T) {
	testCases := []struct {
		name   string
		until  string
		thru   string
		resume bool
	}{
		{"both_until_and_thru", "make_temporary_directories", "calculate_rootfs_size", false},
		{"invalid_until_name", "fake step", "", false},
		{"invalid_thru_name", "", "fake step", false},
		{"resume_with_no_workdir", "", "", true},
	}

	for _, tc := range testCases {
		t.Run("test "+tc.name, func(t *testing.T) {
			var stateMachine StateMachine
			stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
			stateMachine.stateMachineFlags.Until = tc.until
			stateMachine.stateMachineFlags.Thru = tc.thru
			stateMachine.stateMachineFlags.Resume = tc.resume

			if err := stateMachine.validateInput(); err == nil {
				t.Error("Expected an error but there was none!")
			}
		})
	}
}

// TestDebug ensures that the name of the states is printed when the --debug flag is used
func TestDebug(t *testing.T) {
	t.Run("test_debug", func(t *testing.T) {
		workDir := "ubuntu-image-test-debug"
		if err := os.Mkdir("ubuntu-image-test-debug", 0755); err != nil {
			t.Errorf("Failed to create temporary directory %s\n", workDir)
		}

		var stateMachine testStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
		stateMachine.stateMachineFlags.WorkDir = workDir
		stateMachine.commonFlags.Debug = true

		stateMachine.Setup()

		// just use the one state
		stateMachine.states = testStates
		stdout, restoreStdout, err := helper.CaptureStd(&os.Stdout)
		if err != nil {
			t.Errorf("Failed to capture stdout: %s\n", err.Error())
		}

		stateMachine.Run()

		// restore stdout and check that the debug info was printed
		restoreStdout()
		readStdout, err := ioutil.ReadAll(stdout)
		if err != nil {
			t.Errorf("Failed to read stdout: %s\n", err.Error())
		}
		if !strings.Contains(string(readStdout), stateMachine.states[0].name) {
			t.Errorf("Expected state name \"%s\" to appear in output \"%s\"\n", stateMachine.states[0].name, string(readStdout))
		}
		// clean up
		os.RemoveAll(workDir)
	})
}

// TestFunction replaces some of the stateFuncs to test various error scenarios
func TestFunctionErrors(t *testing.T) {
	testCases := []struct {
		name          string
		overrideState int
		newStateFunc  stateFunc
	}{
		{"error_state_func", 0, stateFunc{"test_error_state_func", func(stateMachine *StateMachine) error { return fmt.Errorf("Test Error") }}},
		{"error_write_metadata", 13, stateFunc{"test_error_write_metadata", func(stateMachine *StateMachine) error {
			os.RemoveAll(stateMachine.stateMachineFlags.WorkDir)
			return nil
		}}},
	}
	for _, tc := range testCases {
		t.Run("test "+tc.name, func(t *testing.T) {
			workDir := filepath.Join("/tmp", "ubuntu-image-"+tc.name)
			if err := os.Mkdir(workDir, 0755); err != nil {
				t.Errorf("Failed to create temporary directory %s\n", workDir)
			}

			var stateMachine testStateMachine
			stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
			stateMachine.stateMachineFlags.WorkDir = workDir
			stateMachine.Setup()

			// override the function, but save the old one
			oldStateFunc := stateMachine.states[tc.overrideState]
			stateMachine.states[tc.overrideState] = tc.newStateFunc
			defer func() {
				stateMachine.states[tc.overrideState] = oldStateFunc
			}()
			if err := stateMachine.Run(); err == nil {
				if err := stateMachine.Teardown(); err == nil {
					t.Errorf("Expected an error but there was none")
				}
			}

			// clean up the workdir
			os.RemoveAll(workDir)
		})
	}
}

// TestSetCommonOpts ensures that the function actually sets the correct values in the struct
func TestSetCommonOpts(t *testing.T) {
	t.Run("test_set_common_opts", func(t *testing.T) {
		commonOpts := new(commands.CommonOpts)
		stateMachineOpts := new(commands.StateMachineOpts)
		commonOpts.Debug = true
		stateMachineOpts.WorkDir = testDir

		var stateMachine testStateMachine
		stateMachine.SetCommonOpts(commonOpts, stateMachineOpts)

		if !stateMachine.commonFlags.Debug || stateMachine.stateMachineFlags.WorkDir != testDir {
			t.Error("SetCommonOpts failed to set the correct options")
		}
	})
}

// TestFailedMetadataParse tests a failure in parsing the metadata file. This is accomplished
// by giving the state machine a syntactically invalid metadata file to parse
func TestFailedMetadataParse(t *testing.T) {
	t.Run("test_failed_metadata_parse", func(t *testing.T) {
		var stateMachine StateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
		stateMachine.stateMachineFlags.Resume = true
		stateMachine.stateMachineFlags.WorkDir = "testdata"

		if err := stateMachine.readMetadata(); err == nil {
			t.Errorf("Expected an error but there was none")
		}
	})
}

// TestParseImageSizes tests a successful image size parse with all of the different allowed syntaxes
func TestParseImageSizes(t *testing.T) {
	testCases := []struct {
		name   string
		size   string
		result map[string]quantity.Size
	}{
		{"one_size", "4G", map[string]quantity.Size{
			"first":  4 * quantity.SizeGiB,
			"second": 4 * quantity.SizeGiB,
			"third":  4 * quantity.SizeGiB,
			"fourth": 4 * quantity.SizeGiB}},
		{"size_per_image_name", "first:1G,second:2G,third:3G,fourth:4G", map[string]quantity.Size{
			"first":  1 * quantity.SizeGiB,
			"second": 2 * quantity.SizeGiB,
			"third":  3 * quantity.SizeGiB,
			"fourth": 4 * quantity.SizeGiB}},
		{"size_per_image_number", "0:1G,1:2G,2:3G,3:4G", map[string]quantity.Size{
			"first":  1 * quantity.SizeGiB,
			"second": 2 * quantity.SizeGiB,
			"third":  3 * quantity.SizeGiB,
			"fourth": 4 * quantity.SizeGiB}},
		{"mixed_size_syntax", "0:1G,second:2G,2:3G,fourth:4G", map[string]quantity.Size{
			"first":  1 * quantity.SizeGiB,
			"second": 2 * quantity.SizeGiB,
			"third":  3 * quantity.SizeGiB,
			"fourth": 4 * quantity.SizeGiB}},
	}
	for _, tc := range testCases {
		t.Run("test_parse_image_sizes_"+tc.name, func(t *testing.T) {
			var stateMachine StateMachine
			stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
			stateMachine.yamlFilePath = filepath.Join("testdata", "gadget-multi.yaml")
			stateMachine.commonFlags.Size = tc.size

			// need workdir and loaded gadget.yaml set up for this
			if err := stateMachine.makeTemporaryDirectories(); err != nil {
				t.Errorf("Did not expect an error, got %s", err.Error())
			}
			if err := stateMachine.loadGadgetYaml(); err != nil {
				t.Errorf("Did not expect an error, got %s", err.Error())
			}

			if err := stateMachine.parseImageSizes(); err != nil {
				t.Errorf("Did not expect an error, got %s", err.Error())
			}
			// ensure the correct size was set
			for volumeName := range stateMachine.gadgetInfo.Volumes {
				setSize := stateMachine.imageSizes[volumeName]
				if setSize != tc.result[volumeName] {
					t.Errorf("Volume %s has the wrong size set: %d", volumeName, setSize)
				}
			}

		})
	}
}

// TestFailedParseImageSizes tests failures in parsing the image sizes
func TestFailedParseImageSizes(t *testing.T) {
	testCases := []struct {
		name string
		size string
	}{
		{"invalid_size", "4test"},
		{"too_many_args", "first:1G:2G"},
		{"multiple_invalid", "first:1test"},
		{"volume_not_exist", "fifth:1G"},
		{"index_out_of_range", "9:1G"},
	}
	for _, tc := range testCases {
		t.Run("test_failed_parse_image_sizes_"+tc.name, func(t *testing.T) {
			var stateMachine StateMachine
			stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
			stateMachine.yamlFilePath = filepath.Join("testdata", "gadget-multi.yaml")

			// need workdir and loaded gadget.yaml set up for this
			if err := stateMachine.makeTemporaryDirectories(); err != nil {
				t.Errorf("Did not expect an error, got %s", err.Error())
			}
			if err := stateMachine.loadGadgetYaml(); err != nil {
				t.Errorf("Did not expect an error, got %s", err.Error())
			}

			// run parseImage size and make sure it failed
			stateMachine.commonFlags.Size = tc.size
			if err := stateMachine.parseImageSizes(); err == nil {
				t.Errorf("Expected an error, but got none")
			}
		})
	}
}

// TestHandleContentSizes ensures that using --image-size with a few different values
// results in the correct sizes in stateMachine.imageSizes
func TestHandleContentSizes(t *testing.T) {
	testCases := []struct {
		name   string
		size   string
		result map[string]quantity.Size
	}{
		{"size_not_specified", "", map[string]quantity.Size{"pc": 17825792}},
		{"size_smaller_than_content", "pc:123", map[string]quantity.Size{"pc": 17825792}},
		{"size_bigger_than_content", "pc:4G", map[string]quantity.Size{"pc": 4 * quantity.SizeGiB}},
	}
	for _, tc := range testCases {
		t.Run("test_handle_content_sizes_"+tc.name, func(t *testing.T) {
			var stateMachine StateMachine
			stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
			stateMachine.commonFlags.Size = tc.size
			stateMachine.yamlFilePath = filepath.Join("testdata", "gadget_tree",
				"meta", "gadget.yaml")

			// need workdir and loaded gadget.yaml set up for this
			if err := stateMachine.makeTemporaryDirectories(); err != nil {
				t.Errorf("Did not expect an error, got %s", err.Error())
			}
			if err := stateMachine.loadGadgetYaml(); err != nil {
				t.Errorf("Did not expect an error, got %s", err.Error())
			}

			stateMachine.handleContentSizes(0, "pc")
			// ensure the correct size was set
			for volumeName := range stateMachine.gadgetInfo.Volumes {
				setSize := stateMachine.imageSizes[volumeName]
				if setSize != tc.result[volumeName] {
					t.Errorf("Volume %s has the wrong size set: %d. "+
						"Should be %d", volumeName, setSize, tc.result[volumeName])
				}
			}
		})
	}
}

// TestFailedPostProcessGadgetYaml tests failues in the post processing of
// the gadget.yaml file after loading it in. This is accomplished by mocking
// os.MkdirAll
func TestFailedPostProcessGadgetYaml(t *testing.T) {
	t.Run("test_failed_post_process_gadget_yaml", func(t *testing.T) {
		var stateMachine StateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
		// set a valid yaml file and load it in
		stateMachine.yamlFilePath = filepath.Join("testdata",
			"gadget_tree", "meta", "gadget.yaml")
		// ensure unpack exists
		os.MkdirAll(stateMachine.tempDirs.unpack, 0755)
		if err := stateMachine.loadGadgetYaml(); err != nil {
			t.Errorf("Did not expect an error, got %s", err.Error())
		}

		// mock os.MkdirAll
		osMkdirAll = mockMkdirAll
		defer func() {
			osMkdirAll = os.MkdirAll
		}()
		if err := stateMachine.postProcessGadgetYaml(); err == nil {
			t.Error("Expected an error, but got none")
		}
		osMkdirAll = os.MkdirAll
	})
}