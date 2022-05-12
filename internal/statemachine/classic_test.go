// This test file tests a successful classic run and success/error scenarios for all states
// that are specific to the classic builds
package statemachine

import (
	//"io/ioutil"
	"os"
	//"os/exec"
	"path/filepath"
	//"runtime"
	//"strings"
	"testing"

	"github.com/canonical/ubuntu-image/internal/helper"
	//"github.com/snapcore/snapd/image"
	//"github.com/snapcore/snapd/osutil"
	//"github.com/snapcore/snapd/seed"
	"github.com/xeipuuv/gojsonschema"
)

// TestYAMLSchemaParsing attempts to parse a variety of image definition files, both
// valid and invalid, and ensures the correct result/errors are returned
func TestYAMLSchemaParsing(t *testing.T) {
	testCases := []struct {
		name            string
		imageDefinition string
		shouldPass      bool
		expectedError   string
	}{
		{"valid_image_definition", "test_valid.yaml", true, ""},
		{"invalid_class", "test_bad_class.yaml", false, "Class must be one of the following"},
		{"invalid_url", "test_bad_url.yaml", false, "Does not match format 'uri'"},
		{"both_seed_and_tasks", "test_both_seed_and_tasks.yaml", false, "Must validate one and only one schema"},
		{"git_gadget_without_url", "test_git_gadget_without_url.yaml", false, "When key gadget:type is specified as git, a URL must be provided"},
		{"file_doesnt_exist", "test_not_exist.yaml", false, "no such file or directory"},
		{"not_valid_yaml", "test_invalid_yaml.yaml", false, "yaml: unmarshal errors"},
	}
	for _, tc := range testCases {
		t.Run("test_yaml_schema_"+tc.name, func(t *testing.T) {
			asserter := helper.Asserter{T: t}
			saveCWD := helper.SaveCWD()
			defer saveCWD()

			var stateMachine ClassicStateMachine
			stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
			stateMachine.parent = &stateMachine
			stateMachine.Args.ImageDefinition = filepath.Join("testdata", "image_definitions",
				tc.imageDefinition)
			err := stateMachine.parseImageDefinition()

			if tc.shouldPass {
				asserter.AssertErrNil(err, false)
			} else {
				asserter.AssertErrContains(err, tc.expectedError)
			}
		})
	}
}

// TestFailedParseImageDefinition mocks function calls to test
// failure cases in the parseImageDefinition state
func TestFailedParseImageDefinition(t *testing.T) {
	t.Run("test_failed_parse_image_definition", func(t *testing.T) {
		asserter := helper.Asserter{T: t}
		saveCWD := helper.SaveCWD()
		defer saveCWD()

		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
		stateMachine.parent = &stateMachine
		stateMachine.Args.ImageDefinition = filepath.Join("testdata", "image_definitions",
			"test_valid.yaml")

		// mock gojsonschema.Validate
		gojsonschemaValidate = mockGojsonschemaValidateError
		defer func() {
			gojsonschemaValidate = gojsonschema.Validate
		}()
		err := stateMachine.parseImageDefinition()
		asserter.AssertErrContains(err, "Schema validation returned an error")
		gojsonschemaValidate = gojsonschema.Validate
	})
}

// TestCalculateStates reads in a variety of yaml files and ensures
// that the correct states are added to the state machine
func TestCalculateStates(t *testing.T) {
	testCases := []struct {
		name            string
		imageDefinition string
		expectedStates  []string
	}{
		{"state_build_gadget", "test_build_gadget.yaml", []string{"build_gadget_tree", "load_gadget_yaml"}},
		{"state_prebuilt_gadget", "test_prebuilt_gadget.yaml", []string{"prepare_gadget_tree", "load_gadget_yaml"}},
		{"extract_rootfs_tar", "test_extract_rootfs_tar.yaml", []string{"extract_rootfs_tar"}},
		{"build_rootfs_from_seed", "test_rootfs_seed.yaml", []string{"build_rootfs_from_seed"}},
		{"build_rootfs_from_tasks", "test_rootfs_tasks.yaml", []string{"build_rootfs_from_tasks"}},
		{"customization_states", "test_customization.yaml", []string{"customize_cloud_init", "configure_extra_ppas", "install_extra_packages", "install_extra_snaps", "perform_manual_customization"}},
	}
	for _, tc := range testCases {
		t.Run("test_calcluate_states_"+tc.name, func(t *testing.T) {
			asserter := helper.Asserter{T: t}
			saveCWD := helper.SaveCWD()
			defer saveCWD()

			var stateMachine ClassicStateMachine
			stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
			stateMachine.parent = &stateMachine
			stateMachine.Args.ImageDefinition = filepath.Join("testdata", "image_definitions", tc.imageDefinition)
			err := stateMachine.parseImageDefinition()
			asserter.AssertErrNil(err, true)

			// now calculate the states and ensure that the expected states are in the slice
			err = stateMachine.calculateStates()
			asserter.AssertErrNil(err, true)

			for _, expectedState := range tc.expectedStates {
				stateFound := false
				for _, state := range stateMachine.states {
					if expectedState == state.name {
						stateFound = true
					}
				}
				if !stateFound {
					t.Errorf("state %s should exist in %v, but does not",
						expectedState, stateMachine.states)
				}
			}
		})
	}
}

// TestFailedValidateInputClassic tests a failure in the Setup() function when validating common input
func TestFailedValidateInputClassic(t *testing.T) {
	t.Run("test_failed_validate_input", func(t *testing.T) {
		asserter := helper.Asserter{T: t}
		saveCWD := helper.SaveCWD()
		defer saveCWD()

		// use both --until and --thru to trigger this failure
		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
		stateMachine.stateMachineFlags.Until = "until-test"
		stateMachine.stateMachineFlags.Thru = "thru-test"

		err := stateMachine.Setup()
		asserter.AssertErrContains(err, "cannot specify both --until and --thru")
		os.RemoveAll(stateMachine.stateMachineFlags.WorkDir)
	})
}

// TestFailedReadMetadataClassic tests a failed metadata read by passing --resume with no previous partial state machine run
func TestFailedReadMetadataClassic(t *testing.T) {
	t.Run("test_failed_read_metadata", func(t *testing.T) {
		asserter := helper.Asserter{T: t}
		saveCWD := helper.SaveCWD()
		defer saveCWD()

		// start a --resume with no previous SM run
		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
		stateMachine.stateMachineFlags.Resume = true
		stateMachine.stateMachineFlags.WorkDir = testDir

		err := stateMachine.Setup()
		asserter.AssertErrContains(err, "error reading metadata file")
		os.RemoveAll(stateMachine.stateMachineFlags.WorkDir)
	})
}

// TestPrepareGadgetTree runs prepareGadgetTree() and ensures the gadget_tree files
// are placed in the correct locations
func TestPrepareGadgetTree(t *testing.T) {
	t.Run("test_prepare_gadget_tree", func(t *testing.T) {
		asserter := helper.Asserter{T: t}
		saveCWD := helper.SaveCWD()
		defer saveCWD()

		// use both --until and --thru to trigger this failure
		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()

		err := stateMachine.prepareGadgetTree()
		asserter.AssertErrNil(err, true)
	})
}

// TestFailedPrepareGadgetTree tests failures in os, osutil, and ioutil libraries
func TestFailedPrepareGadgetTree(t *testing.T) {
	t.Run("test_failed_prepare_gadget_tree", func(t *testing.T) {
		// currently a no-op, waiting for prepareGadgetTree
		// to be converted to the new ubuntu-image classic
		// design. This will have ubuntu-image build the
		// gadget tree rather than relying on the user
		// to have done this ahead of time
		t.Skip()
	})
}

// TestBuildGadgetTree unit tests the buildGadgetTree function
func TestBuildGadgetTree(t *testing.T) {
	t.Run("test_build_gadget_tree", func(t *testing.T) {
		asserter := helper.Asserter{T: t}
		saveCWD := helper.SaveCWD()
		defer saveCWD()

		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()

		err := stateMachine.buildGadgetTree()
		asserter.AssertErrNil(err, true)
	})
}

// TestBuildRootfsFromSeed unit tests the buildRootfsFromSeed function
func TestBuildRootfsFromSeed(t *testing.T) {
	t.Run("test_build_rootfs_from_seed", func(t *testing.T) {
		asserter := helper.Asserter{T: t}
		saveCWD := helper.SaveCWD()
		defer saveCWD()

		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()

		err := stateMachine.buildRootfsFromSeed()
		asserter.AssertErrNil(err, true)
	})
}

// TestBuildRootfsFromTasks unit tests the buildRootfsFromTasks function
func TestBuildRootfsFromTasks(t *testing.T) {
	t.Run("test_build_rootfs_from_tasks", func(t *testing.T) {
		asserter := helper.Asserter{T: t}
		saveCWD := helper.SaveCWD()
		defer saveCWD()

		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()

		err := stateMachine.buildRootfsFromTasks()
		asserter.AssertErrNil(err, true)
	})
}

// TestExtractRootfsTar unit tests the extractRootfsTar function
func TestExtractRootfsTar(t *testing.T) {
	t.Run("test_extract_rootfs_tar", func(t *testing.T) {
		asserter := helper.Asserter{T: t}
		saveCWD := helper.SaveCWD()
		defer saveCWD()

		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()

		err := stateMachine.extractRootfsTar()
		asserter.AssertErrNil(err, true)
	})
}

// TestCustomizeCloudInit unit tests the customizeCloudInit function
func TestCustomizeCloudInit(t *testing.T) {
	t.Run("test_customize_cloud_init", func(t *testing.T) {
		asserter := helper.Asserter{T: t}
		saveCWD := helper.SaveCWD()
		defer saveCWD()

		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()

		err := stateMachine.customizeCloudInit()
		asserter.AssertErrNil(err, true)
	})
}

// TestSetupExtraPPAs unit tests the setupExtraPPAs function
func TestSetupExtraPPAs(t *testing.T) {
	t.Run("test_setup_extra_PPAs", func(t *testing.T) {
		asserter := helper.Asserter{T: t}
		saveCWD := helper.SaveCWD()
		defer saveCWD()

		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()

		err := stateMachine.setupExtraPPAs()
		asserter.AssertErrNil(err, true)
	})
}

// TestInstallExtraPackages unit tests the installExtraPackages function
func TestInstallExtraPackages(t *testing.T) {
	t.Run("test_install_extra_packages", func(t *testing.T) {
		asserter := helper.Asserter{T: t}
		saveCWD := helper.SaveCWD()
		defer saveCWD()

		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()

		err := stateMachine.installExtraPackages()
		asserter.AssertErrNil(err, true)
	})
}

// TestManualCustomization unit tests the manualCustomization function
func TestManualCustomization(t *testing.T) {
	t.Run("test_manual_customization", func(t *testing.T) {
		asserter := helper.Asserter{T: t}
		saveCWD := helper.SaveCWD()
		defer saveCWD()

		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()

		err := stateMachine.manualCustomization()
		asserter.AssertErrNil(err, true)
	})
}

// TODO replace this with fakeExecCommand that sil2100 wrote
// TestFailedLiveBuildCommands tests the scenario where calls to `lb` fail
// this is accomplished by temporarily replacing lb on disk with a test script
/*func TestFailedLiveBuildCommands(t *testing.T) {
	testCases := []struct {
		name       string
		testScript string
	}{
		{"failed_lb_config", "lb_config_fail"},
		{"failed_lb_build", "lb_build_fail"},
	}
	for _, tc := range testCases {
		t.Run("test_"+tc.name, func(t *testing.T) {
			asserter := helper.Asserter{T: t}
			saveCWD := helper.SaveCWD()
			defer saveCWD()

			var stateMachine ClassicStateMachine
			stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
			stateMachine.Opts.Project = "ubuntu-cpc"
			stateMachine.Opts.Subproject = "fakeproject"
			stateMachine.Opts.Subarch = "fakearch"
			stateMachine.Opts.WithProposed = true
			stateMachine.Opts.ExtraPPAs = []string{"ppa:fake_user/fakeppa"}
			stateMachine.Args.GadgetTree = filepath.Join("testdata", "gadget_tree")
			stateMachine.parent = &stateMachine

			scriptPath := filepath.Join("testscripts", tc.testScript)
			// save the original lb
			whichLb := *exec.Command("which", "lb")
			lbLocationBytes, _ := whichLb.Output()
			lbLocation := strings.TrimSpace(string(lbLocationBytes))
			// ensure the backup doesn't exist
			os.Remove(lbLocation + ".bak")
			err := os.Rename(lbLocation, lbLocation+".bak")
			asserter.AssertErrNil(err, true)

			err = osutil.CopyFile(scriptPath, lbLocation, 0)
			asserter.AssertErrNil(err, true)
			defer func() {
				os.Remove(lbLocation)
				os.Rename(lbLocation+".bak", lbLocation)
			}()

			// need workdir set up for this
			err = stateMachine.makeTemporaryDirectories()
			asserter.AssertErrNil(err, true)

			// also need unpack set up
			err = os.Mkdir(stateMachine.tempDirs.unpack, 0755)
			asserter.AssertErrNil(err, true)

			err = stateMachine.runLiveBuild()
			asserter.AssertErrContains(err, "Error running command")
			os.RemoveAll(stateMachine.stateMachineFlags.WorkDir)
		})
	}
}

// TestNoStatic tests that the helper function to prepare lb commands
// returns an error if the qemu-static binary is missing. This is accomplished
// by passing an architecture for which there is no qemu-static binary
func TestNoStatic(t *testing.T) {
	t.Run("test_no_qemu_static", func(t *testing.T) {
		asserter := helper.Asserter{T: t}
		saveCWD := helper.SaveCWD()
		defer saveCWD()

		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
		stateMachine.Opts.Project = "ubuntu-cpc"
		stateMachine.Opts.Arch = "fakearch"
		stateMachine.Args.GadgetTree = filepath.Join("testdata", "gadget_tree")
		stateMachine.parent = &stateMachine

		// need workdir set up for this
		err := stateMachine.makeTemporaryDirectories()
		asserter.AssertErrNil(err, true)

		// also need unpack set up
		err = os.Mkdir(stateMachine.tempDirs.unpack, 0755)
		asserter.AssertErrNil(err, true)

		err = stateMachine.runLiveBuild()
		asserter.AssertErrContains(err, "in case of non-standard archs or custom paths")
		os.RemoveAll(stateMachine.stateMachineFlags.WorkDir)
	})
}

// TestPopulateClassicRootfsContents runs the state machine through populate_rootfs_contents and examines
// the rootfs to ensure at least some of the correct file are in place
func TestPopulateClassicRootfsContents(t *testing.T) {
	t.Run("test_populate_classic_rootfs_contents", func(t *testing.T) {
		if runtime.GOARCH != "amd64" {
			t.Skip("Test for amd64 only")
		}
		asserter := helper.Asserter{T: t}
		saveCWD := helper.SaveCWD()
		defer saveCWD()

		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
		stateMachine.parent = &stateMachine
		stateMachine.Opts.Project = "ubuntu-cpc"
		stateMachine.Opts.Suite = "focal"
		stateMachine.Args.GadgetTree = filepath.Join("testdata", "gadget_tree")
		stateMachine.commonFlags.Snaps = []string{"hello", "ubuntu-image/classic=edge", "core20=beta"}
		stateMachine.stateMachineFlags.Thru = "populate_rootfs_contents"

		err := stateMachine.Setup()
		asserter.AssertErrNil(err, true)

		err = stateMachine.Run()
		asserter.AssertErrNil(err, true)

		// check the files before Teardown
		fileList := []string{filepath.Join("etc", "shadow"),
			filepath.Join("etc", "systemd"),
			filepath.Join("boot", "vmlinuz"),
			filepath.Join("boot", "grub"),
			filepath.Join("usr", "lib")}
		for _, file := range fileList {
			_, err := os.Stat(filepath.Join(stateMachine.tempDirs.rootfs, file))
			if err != nil {
				if os.IsNotExist(err) {
					t.Errorf("File %s should exist, but does not", file)
				}
			}
		}

		// check /etc/fstab contents to test the scenario where the regex replaced an
		// existing filesystem label with LABEL=writable
		fstab, err := ioutilReadFile(filepath.Join(stateMachine.tempDirs.rootfs,
			"etc", "fstab"))
		if err != nil {
			t.Errorf("Error reading fstab to check regex")
		}
		correctLabel := "LABEL=writable"
		if !strings.Contains(string(fstab), correctLabel) {
			t.Errorf("Expected fstab contents %s to contain %s",
				string(fstab), correctLabel)
		}

		// check that extra snaps were added to the rootfs
		for _, snap := range stateMachine.commonFlags.Snaps {
			if strings.Contains(snap, "/") {
				snap = strings.Split(snap, "/")[0]
			}
			if strings.Contains(snap, "=") {
				snap = strings.Split(snap, "=")[0]
			}
			filePath := filepath.Join(stateMachine.tempDirs.rootfs,
				"var", "snap", snap)
			if !osutil.FileExists(filePath) {
				t.Errorf("File %s should exist but it does not", filePath)
			}
		}

		err = stateMachine.Teardown()
		asserter.AssertErrNil(err, false)
	})
}

// TestFailedPopulateClassicRootfsContents tests failed scenarios in populateClassicRootfsContents
// this is accomplished by mocking functions
func TestFailedPopulateClassicRootfsContents(t *testing.T) {
	t.Run("test_failed_populate_classic_rootfs_contents", func(t *testing.T) {
		asserter := helper.Asserter{T: t}
		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
		stateMachine.parent = &stateMachine
		stateMachine.Opts.Filesystem = filepath.Join("testdata", "filesystem")
		stateMachine.commonFlags.CloudInit = filepath.Join("testdata", "user-data")

		// need workdir set up for this
		err := stateMachine.makeTemporaryDirectories()
		asserter.AssertErrNil(err, true)

		// mock ioutil.ReadDir
		ioutilReadDir = mockReadDir
		defer func() {
			ioutilReadDir = ioutil.ReadDir
		}()
		err = stateMachine.populateClassicRootfsContents()
		asserter.AssertErrContains(err, "Error reading unpack/chroot dir")
		ioutilReadDir = ioutil.ReadDir

		// mock osutil.CopySpecialFile
		osutilCopySpecialFile = mockCopySpecialFile
		defer func() {
			osutilCopySpecialFile = osutil.CopySpecialFile
		}()
		err = stateMachine.populateClassicRootfsContents()
		asserter.AssertErrContains(err, "Error copying rootfs")
		osutilCopySpecialFile = osutil.CopySpecialFile

		// mock ioutil.WriteFile
		ioutilWriteFile = mockWriteFile
		defer func() {
			ioutilWriteFile = ioutil.WriteFile
		}()
		err = stateMachine.populateClassicRootfsContents()
		asserter.AssertErrContains(err, "Error writing to fstab")
		ioutilWriteFile = ioutil.WriteFile

		// mock os.MkdirAll
		osMkdirAll = mockMkdirAll
		defer func() {
			osMkdirAll = os.MkdirAll
		}()
		err = stateMachine.populateClassicRootfsContents()
		asserter.AssertErrContains(err, "Error creating cloud-init dir")
		osMkdirAll = os.MkdirAll

		// mock os.OpenFile
		osOpenFile = mockOpenFile
		defer func() {
			osOpenFile = os.OpenFile
		}()
		err = stateMachine.populateClassicRootfsContents()
		asserter.AssertErrContains(err, "Error opening cloud-init meta-data file")
		osOpenFile = os.OpenFile

		// mock osutil.CopyFile
		osutilCopyFile = mockCopyFile
		defer func() {
			osutilCopyFile = osutil.CopyFile
		}()
		err = stateMachine.populateClassicRootfsContents()
		asserter.AssertErrContains(err, "Error copying cloud-init")
		osutilCopyFile = osutil.CopyFile
	})
}

// TestFilesystemFlag makes sure that with the --filesystem flag the specified filesystem is copied
// to the rootfs directory
func TestFilesystemFlag(t *testing.T) {
	t.Run("test_filesystem_flag", func(t *testing.T) {
		asserter := helper.Asserter{T: t}
		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
		stateMachine.parent = &stateMachine
		stateMachine.Opts.Filesystem = filepath.Join("testdata", "filesystem")

		// need workdir set up for this
		err := stateMachine.makeTemporaryDirectories()
		asserter.AssertErrNil(err, true)

		err = stateMachine.populateClassicRootfsContents()
		asserter.AssertErrNil(err, true)

		// check that the specified filesystem was copied over
		if _, err := os.Stat(filepath.Join(stateMachine.tempDirs.rootfs, "testfile")); err != nil {
			t.Errorf("Failed to copy --filesystem to rootfs")
		}

		// the included filesystem contains an invalid /etc/fstab. Make sure it
		// was overwritten to have a valid /etc/fstab
		fstab, err := ioutilReadFile(filepath.Join(stateMachine.tempDirs.rootfs,
			"etc", "fstab"))
		if err != nil {
			t.Errorf("Error reading fstab to check regex")
		}
		correctLabel := "LABEL=writable   /    ext4   defaults    0 0"
		if !strings.Contains(string(fstab), correctLabel) {
			t.Errorf("Expected fstab contents %s to contain %s",
				string(fstab), correctLabel)
		}
	})
}

// TestGeneratePackageManifest tests if classic image manifest generation works
func TestGeneratePackageManifest(t *testing.T) {
	t.Run("test_generate_package_manifest", func(t *testing.T) {
		asserter := helper.Asserter{T: t}

		// Setup the exec.Command mock
		testCaseName = "TestGeneratePackageManifest"
		execCommand = fakeExecCommand
		defer func() {
			execCommand = exec.Command
		}()
		// We need the output directory set for this
		outputDir, err := ioutil.TempDir("/tmp", "ubuntu-image-")
		asserter.AssertErrNil(err, true)
		defer os.RemoveAll(outputDir)

		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
		stateMachine.parent = &stateMachine
		stateMachine.commonFlags.OutputDir = outputDir
		osMkdirAll(stateMachine.commonFlags.OutputDir, 0755)

		err = stateMachine.generatePackageManifest()
		asserter.AssertErrNil(err, true)

		// Check if manifest file got generated and if it has expected contents
		manifestPath := filepath.Join(stateMachine.commonFlags.OutputDir, "filesystem.manifest")
		manifestBytes, err := ioutil.ReadFile(manifestPath)
		asserter.AssertErrNil(err, true)
		// The order of packages shouldn't matter
		examplePackages := []string{"foo 1.2", "bar 1.4-1ubuntu4.1", "libbaz 0.1.3ubuntu2"}
		for _, pkg := range examplePackages {
			if !strings.Contains(string(manifestBytes), pkg) {
				t.Errorf("filesystem.manifest does not contain expected package: %s", pkg)
			}
		}
	})
}

// TestFailedGeneratePackageManifest tests if classic manifest generation failures are reported
func TestFailedGeneratePackageManifest(t *testing.T) {
	t.Run("test_failed_generate_package_manifest", func(t *testing.T) {
		asserter := helper.Asserter{T: t}

		// Setup the exec.Command mock - version from the success test
		testCaseName = "TestGeneratePackageManifest"
		execCommand = fakeExecCommand
		defer func() {
			execCommand = exec.Command
		}()
		// Setup the mock for os.Create, making those fail
		osCreate = mockCreate
		defer func() {
			osCreate = os.Create
		}()

		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
		stateMachine.parent = &stateMachine
		stateMachine.commonFlags.OutputDir = "/test/path"

		err := stateMachine.generatePackageManifest()
		asserter.AssertErrContains(err, "Error creating manifest file")
	})
}

// TestFailedRunLiveBuild tests some error scenarios in the runLiveBuild state that are not
// caused by actual failures in the `lb` commands
func TestFailedRunLiveBuild(t *testing.T) {
	t.Run("test_failed_run_live_build", func(t *testing.T) {
		asserter := helper.Asserter{T: t}

		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
		stateMachine.parent = &stateMachine
		stateMachine.Opts.Project = "ubuntu-cpc"
		stateMachine.Opts.Suite = "focal"
		stateMachine.Args.GadgetTree = filepath.Join("testdata", "gadget_tree")
		stateMachine.commonFlags.Snaps = []string{"hello", "ubuntu-image/classic", "core20=beta"}
		stateMachine.stateMachineFlags.Thru = "run_live_build"

		// replace the lb commands with a script that will simply pass
		testCaseName = "TestFailedRunLiveBuild"
		execCommand = fakeExecCommand
		defer func() {
			execCommand = exec.Command
		}()
		// since we have mocked exec.Command, running dpkg -L to find the livecd-rootfs
		// filepath will fail. We can use an environment variable instead
		dpkgArgs := "dpkg -L livecd-rootfs | grep \"auto$\""
		dpkgCommand := *exec.Command("bash", "-c", dpkgArgs)
		dpkgBytes, err := dpkgCommand.Output()
		asserter.AssertErrNil(err, true)
		autoSrc := strings.TrimSpace(string(dpkgBytes))
		os.Setenv("UBUNTU_IMAGE_LIVECD_ROOTFS_AUTO_PATH", autoSrc)

		// mock os.OpenFile
		osOpenFile = mockOpenFile
		defer func() {
			osOpenFile = os.OpenFile
		}()
		err = stateMachine.Setup()
		asserter.AssertErrNil(err, true)

		err = stateMachine.Run()
		asserter.AssertErrContains(err, "Error opening seeded-snaps")
		osOpenFile = os.OpenFile
		os.RemoveAll(testDir)

		// mock os.OpenFile
		osOpenFile = mockOpenFileBadPerms
		defer func() {
			osOpenFile = os.OpenFile
		}()
		err = stateMachine.Setup()
		asserter.AssertErrNil(err, true)

		err = stateMachine.Run()
		asserter.AssertErrContains(err, "Error writing snap hello=stable to seeded-snaps")
		osOpenFile = os.OpenFile
		os.RemoveAll(testDir)
		os.Unsetenv("UBUNTU_IMAGE_LIVECD_ROOTFS_AUTO_PATH")
	})
}

// TestExtraSnapsWithFilesystem tests that using --snap along with --filesystem preseeds the snaps
// in the resulting root filesystem
func TestExtraSnapsWithFilesystem(t *testing.T) {
	t.Run("test_extra_snaps_with_filesystem", func(t *testing.T) {
		if runtime.GOARCH != "amd64" {
			t.Skip("Test for amd64 only")
		}
		asserter := helper.Asserter{T: t}
		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
		stateMachine.parent = &stateMachine
		stateMachine.Opts.Filesystem = filepath.Join("testdata", "filesystem")
		stateMachine.commonFlags.Snaps = []string{"hello"}

		// need workdir set up for this
		err := stateMachine.makeTemporaryDirectories()
		asserter.AssertErrNil(err, true)

		// copy the filesystem over before attempting to preseed it
		err = stateMachine.populateClassicRootfsContents()
		asserter.AssertErrNil(err, true)

		// call "snap prepare image" to preseed the filesystem.
		// Doing the preseed at the time of the test allows it to
		// run on each architecture and keeps the github repository
		// free of large .snap files
		snapPrepareImage := *exec.Command("snap", "prepare-image", "--arch=amd64",
			"--classic", "--snap=core20", "--snap=snapd", "--snap=lxd",
			filepath.Join("testdata", "modelAssertionClassic"),
			stateMachine.tempDirs.rootfs)
		err = snapPrepareImage.Run()
		asserter.AssertErrNil(err, true)

		// now call prepateClassicImage to simulate using --snap with --filesystem
		err = stateMachine.prepareClassicImage()
		asserter.AssertErrNil(err, true)

		// Ensure that the hello snap was preseeded in the filesystem and the
		// snaps that were already there haven't been removed
		snapList := []string{"hello", "lxd", "core20", "snapd"}
		for _, snap := range snapList {
			snapGlob := filepath.Join(stateMachine.tempDirs.rootfs,
				"var", "lib", "snapd", "snaps", snap+"*.snap")
			snapFile, _ := filepath.Glob(snapGlob)
			if len(snapFile) == 0 {
				if os.IsNotExist(err) {
					t.Errorf("File %s should exist, but does not", snapGlob)
				}
			}
		}
	})
}

// TestFailedPrepareClassiImage tests various failure scenarios in the prepateClassicImage function
func TestFailedPrepareClassicImage(t *testing.T) {
	t.Run("test_failed_prepare_classic_image", func(t *testing.T) {
		asserter := helper.Asserter{T: t}
		var stateMachine ClassicStateMachine
		stateMachine.commonFlags, stateMachine.stateMachineFlags = helper.InitCommonOpts()
		stateMachine.parent = &stateMachine
		stateMachine.Opts.Filesystem = filepath.Join("testdata", "filesystem")

		// need workdir set up for this
		err := stateMachine.makeTemporaryDirectories()
		asserter.AssertErrNil(err, true)

		// copy the filesystem over before attempting to preseed it
		err = stateMachine.populateClassicRootfsContents()
		asserter.AssertErrNil(err, true)

		// call "snap prepare image" to preseed the filesystem.
		// Doing the preseed at the time of the test allows it to
		// run on each architecture and keeps the github repository
		// free of large .snap files
		snapPrepareImage := *exec.Command("snap", "prepare-image", "--arch=amd64",
			"--classic", "--snap=core20", "--snap=snapd", "--snap=lxd",
			filepath.Join("testdata", "modelAssertionClassic"),
			stateMachine.tempDirs.rootfs)
		err = snapPrepareImage.Run()
		asserter.AssertErrNil(err, true)

		// set an invalid value for --snap to cause an error in
		// parseSnapsAndChannels
		stateMachine.commonFlags.Snaps = []string{"hello=test=invalid"}
		err = stateMachine.prepareClassicImage()
		asserter.AssertErrContains(err, "Invalid syntax passed to --snap")
		os.RemoveAll(filepath.Join(stateMachine.stateMachineFlags.WorkDir, "model"))

		// set a valid value for --snap and mock seed.Open to simulate
		// a failure reading the seed
		stateMachine.commonFlags.Snaps = []string{"hello"}
		seedOpen = mockSeedOpen
		defer func() {
			seedOpen = seed.Open
		}()
		err = stateMachine.prepareClassicImage()
		asserter.AssertErrContains(err, "Error removing preseeded snaps")
		seedOpen = seed.Open
		os.RemoveAll(filepath.Join(stateMachine.stateMachineFlags.WorkDir, "model"))

		// mock osutil.CopyFile
		osutilCopyFile = mockCopyFile
		defer func() {
			osutilCopyFile = osutil.CopyFile
		}()
		err = stateMachine.prepareClassicImage()
		asserter.AssertErrContains(err, "Error copying model")
		osutilCopyFile = osutil.CopyFile
		os.RemoveAll(filepath.Join(stateMachine.stateMachineFlags.WorkDir, "model"))

		// mock image.Prepare
		stateMachine.commonFlags.Snaps = []string{"hello"}
		imagePrepare = mockImagePrepare
		defer func() {
			imagePrepare = image.Prepare
		}()
		err = stateMachine.prepareClassicImage()
		asserter.AssertErrContains(err, "Error preparing image")
		imagePrepare = image.Prepare

		os.RemoveAll(stateMachine.stateMachineFlags.WorkDir)
	})
}*/
