package helper

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/snapcore/snapd/osutil"
)

// define some mocked versions of go package functions
func mockRemove(string) error {
	return fmt.Errorf("Test Error")
}
func mockRename(string, string) error {
	return fmt.Errorf("Test Error")
}

// TestRestoreResolvConf tests if resolv.conf is restored correctly
func TestRestoreResolvConf(t *testing.T) {
	t.Run("test_restore_resolv_conf", func(t *testing.T) {
		asserter := Asserter{T: t}
		// Prepare temporary directory
		workDir := filepath.Join("/tmp", "ubuntu-image-"+uuid.NewString())
		err := os.Mkdir(workDir, 0755)
		asserter.AssertErrNil(err, true)
		defer os.RemoveAll(workDir)

		// Create test objects for a regular backup
		err = os.MkdirAll(filepath.Join(workDir, "etc"), 0755)
		asserter.AssertErrNil(err, true)
		mainConfPath := filepath.Join(workDir, "etc", "resolv.conf")
		mainConf, err := os.Create(mainConfPath)
		asserter.AssertErrNil(err, true)
		testData := []byte("Main")
		_, err = mainConf.Write(testData)
		asserter.AssertErrNil(err, true)
		mainConf.Close()
		backupConfPath := filepath.Join(workDir, "etc", "resolv.conf.tmp")
		backupConf, err := os.Create(backupConfPath)
		asserter.AssertErrNil(err, true)
		testData = []byte("Backup")
		_, err = backupConf.Write(testData)
		asserter.AssertErrNil(err, true)
		backupConf.Close()

		err = RestoreResolvConf(workDir)
		asserter.AssertErrNil(err, true)
		if osutil.FileExists(backupConfPath) {
			t.Errorf("Backup resolv.conf.tmp has not been removed")
		}
		checkData, err := os.ReadFile(mainConfPath)
		asserter.AssertErrNil(err, true)
		if string(checkData) != "Backup" {
			t.Errorf("Main resolv.conf has not been restored")
		}

		// Now check if the symlink case also works
		_, err = os.Create(backupConfPath)
		asserter.AssertErrNil(err, true)
		err = os.Remove(mainConfPath)
		asserter.AssertErrNil(err, true)
		err = os.Symlink("resolv.conf.tmp", mainConfPath)
		asserter.AssertErrNil(err, true)

		err = RestoreResolvConf(workDir)
		asserter.AssertErrNil(err, true)
		if osutil.FileExists(backupConfPath) {
			t.Errorf("Backup resolv.conf.tmp has not been removed when dealing with as symlink")
		}
		if !osutil.IsSymlink(mainConfPath) {
			t.Errorf("Main resolv.conf should have remained a symlink, but it is not")
		}
	})
}

// TestFailedRestoreResolvConf tests all resolv.conf error cases
func TestFailedRestoreResolvConf(t *testing.T) {
	t.Run("test_failed_restore_resolv_conf", func(t *testing.T) {
		asserter := Asserter{T: t}
		// Prepare temporary directory
		workDir := filepath.Join("/tmp", "ubuntu-image-"+uuid.NewString())
		err := os.Mkdir(workDir, 0755)
		asserter.AssertErrNil(err, true)
		defer os.RemoveAll(workDir)

		// Create test environment
		err = os.MkdirAll(filepath.Join(workDir, "etc"), 0755)
		asserter.AssertErrNil(err, true)
		mainConfPath := filepath.Join(workDir, "etc", "resolv.conf")
		_, err = os.Create(mainConfPath)
		asserter.AssertErrNil(err, true)
		backupConfPath := filepath.Join(workDir, "etc", "resolv.conf.tmp")
		_, err = os.Create(backupConfPath)
		asserter.AssertErrNil(err, true)

		// Mock the os.Rename failure
		osRename = mockRename
		defer func() {
			osRename = os.Rename
		}()
		err = RestoreResolvConf(workDir)
		asserter.AssertErrContains(err, "Error moving file")

		// Mock the os.Remove failure
		err = os.Remove(mainConfPath)
		asserter.AssertErrNil(err, true)
		err = os.Symlink("resolv.conf.tmp", mainConfPath)
		asserter.AssertErrNil(err, true)
		osRemove = mockRemove
		defer func() {
			osRemove = os.Remove
		}()
		err = RestoreResolvConf(workDir)
		asserter.AssertErrContains(err, "Error removing file")
	})
}

type S1 struct {
	A string `default:"test"`
	B string
	C []string `default:"1,2,3"`
	D []*S3
	E *S3
}

type S2 struct {
	A string `default:"test"`
	B *bool  `default:"true"`
	C bool   `default:"true"`
	D bool   `default:"true"`
	E *bool  `default:"false"`
	F bool   `default:"false"`
	G *bool
}

type S3 struct {
	A string `default:"defaults3value"`
}

type S4 struct {
	A int `default:"1"`
}

type S5 struct {
	A *S4
}

type S6 struct {
	A []*S4
}

func TestSetDefaults(t *testing.T) {
	type args struct {
		needsDefaults interface{}
	}
	tests := []struct {
		name          string
		args          args
		want          interface{}
		wantErr       bool
		expectedError string
	}{
		{
			name: "set default on empty struct",
			args: args{
				needsDefaults: &S1{},
			},
			want: &S1{
				A: "test",
				B: "",
				C: []string{"1", "2", "3"},
			},
		},
		{
			name: "set default on non-empty struct",
			args: args{
				needsDefaults: &S1{
					A: "non-empty-A-value",
					B: "non-empty-B-value",
					C: []string{"non-empty-C-value"},
					D: []*S3{
						{
							A: "non-empty-A-value",
						},
					},
					E: &S3{
						A: "non-empty-A-value",
					},
				},
			},
			want: &S1{
				A: "non-empty-A-value",
				B: "non-empty-B-value",
				C: []string{"non-empty-C-value"},
				D: []*S3{
					{
						A: "non-empty-A-value",
					},
				},
				E: &S3{
					A: "non-empty-A-value",
				},
			},
		},
		{
			name: "set default on empty struct with bool",
			args: args{
				needsDefaults: &S2{},
			},
			want: &S2{
				A: "test",
				B: BoolPtr(true),
				C: true,
				D: true,
				E: BoolPtr(false),
				F: false,
				G: BoolPtr(false), // even default values we do not let nil pointer
			},
		},
		{
			name: "set default on non-empty struct with bool",
			args: args{
				needsDefaults: &S2{
					B: BoolPtr(false),
					D: false,
					F: true,
					G: BoolPtr(true),
				},
			},
			want: &S2{
				A: "test",
				B: BoolPtr(true),
				C: true,
				D: true, //shows that default values on bools do not work
				// properly without using a pointer to bool
				E: BoolPtr(false),
				F: true,
				G: BoolPtr(true),
			},
		},
		{
			name: "fail to set default on struct with unsuported type",
			args: args{
				needsDefaults: &S4{},
			},
			expectedError: "not supported",
		},
		{
			name: "fail to set default on struct containing a struct with unsuported type",
			args: args{
				needsDefaults: &S5{
					A: &S4{},
				},
			},
			expectedError: "not supported",
		},
		{
			name: "fail to set default on struct containing an slice of struct with unsuported type",
			args: args{
				needsDefaults: &S6{
					A: []*S4{
						{},
					},
				},
			},
			expectedError: "not supported",
		},
		{
			name: "fail to set default on a concrete object (not a pointer)",
			args: args{
				needsDefaults: S1{},
			},
			expectedError: "The argument to SetDefaults must be a pointer",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			asserter := Asserter{T: t}
			err := SetDefaults(tc.args.needsDefaults)

			if len(tc.expectedError) == 0 {
				asserter.AssertErrNil(err, true)
				asserter.AssertEqual(tc.want, tc.args.needsDefaults)
			} else {
				asserter.AssertErrContains(err, tc.expectedError)
			}

		})
	}
}
