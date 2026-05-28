package btrfs

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestParseFilesystemShow(t *testing.T) {
	output := `Label: 'tank'  uuid: 11111111-2222-3333-4444-555555555555
	Total devices 2 FS bytes used 512.00MiB
	devid    1 size 1073741824 used 805306368 path /dev/sda1
	devid    2 size 1073741824 used 268435456 path /dev/sdb1
`

	var fs Filesystem
	err := parseFilesystemShow(&fs, output)
	require.NoError(t, err)
	require.Equal(t, "tank", fs.Label)
	require.Equal(t, "11111111-2222-3333-4444-555555555555", fs.UUID)
	require.Len(t, fs.Devices, 2)
	require.Equal(t, "/dev/sda1", fs.Devices[0].Path)
}

func TestParseFilesystemUsage(t *testing.T) {
	output := `Overall:
    Device size:                   2147483648
    Device allocated:              1073741824
    Device unallocated:            1073741824
    Device missing:                0
    Used:                          805306368
    Free (estimated):              1342177280      (min: 1342177280)
    Free (statfs, df):             1207959552
    Data ratio:                    1.00
    Metadata ratio:                2.00
    Multiple profiles:             no
Data, single: total=536870912, used=268435456
Metadata, DUP: total=268435456, used=134217728
System, DUP: total=33554432, used=16384
`

	var fs Filesystem
	err := parseFilesystemUsage(&fs, output)
	require.NoError(t, err)
	require.Equal(t, int64(2147483648), fs.DeviceSize)
	require.Equal(t, int64(1073741824), fs.DeviceAllocated)
	require.Equal(t, "single", fs.DataProfile)
	require.Equal(t, "DUP", fs.MetadataProfile)
	require.InDelta(t, 1.0, fs.DataRatio, 0.001)
	require.False(t, fs.MultipleProfiles)
}

func TestParseFilesystemUsageSynologyDSM7(t *testing.T) {
	output := `Overall:
    Device size:                      468151435264
    Device allocated:                 240543334400
    Device unallocated:               227608100864
    Device missing:                              0
    Used:                              60292476928
    Free (estimated):                 392650477568      (min: 278846427136)
    Data ratio:                               1.00
    Metadata ratio:                           2.00
    Global reserve:                      268435456      (used: 0)

Data,single: Size:223346688000, Used:58304311296
   /dev/vg1/volume_1    223346688000

Metadata,DUP: Size:8589934592, Used:994033664
   /dev/vg1/volume_1    17179869184

System,DUP: Size:8388608, Used:49152
   /dev/vg1/volume_1      16777216

Unallocated:
   /dev/vg1/volume_1    227608100864
`

	var fs Filesystem
	err := parseFilesystemUsage(&fs, output)
	require.NoError(t, err)
	require.Equal(t, int64(468151435264), fs.DeviceSize)
	require.Equal(t, int64(240543334400), fs.DeviceAllocated)
	require.Equal(t, int64(227608100864), fs.DeviceUnallocated)
	require.Equal(t, int64(60292476928), fs.Used)
	require.Equal(t, int64(392650477568), fs.FreeEstimated)
	require.Equal(t, int64(278846427136), fs.FreeMin)
	require.Equal(t, "single", fs.DataProfile)
	require.Equal(t, int64(223346688000), fs.DataTotal)
	require.Equal(t, int64(58304311296), fs.DataUsed)
	require.Equal(t, "DUP", fs.MetadataProfile)
	require.Equal(t, int64(8589934592), fs.MetadataTotal)
	require.Equal(t, int64(994033664), fs.MetadataUsed)
	require.Equal(t, "DUP", fs.SystemProfile)
	require.Equal(t, int64(8388608), fs.SystemTotal)
	require.Equal(t, int64(49152), fs.SystemUsed)
}

func TestParseDeviceStats(t *testing.T) {
	fs := Filesystem{
		Devices: []Device{
			{Path: "/dev/sda1"},
		},
	}
	output := `[/dev/sda1].write_io_errs   1
[/dev/sda1].read_io_errs    2
[/dev/sda1].flush_io_errs   3
[/dev/sda1].corruption_errs 4
[/dev/sda1].generation_errs 5
`

	parseDeviceStats(&fs, output)
	require.Equal(t, int64(1), fs.Devices[0].WriteIOErrors)
	require.Equal(t, int64(2), fs.Devices[0].ReadIOErrors)
	require.Equal(t, int64(5), fs.Devices[0].GenerationErrors)
}

func TestParseScrubStatusFinished(t *testing.T) {
	fs := Filesystem{}
	output := `UUID:             11111111-2222-3333-4444-555555555555
Scrub started:    Wed Apr 10 12:34:56 2024
Status:           finished
Duration:         0:10:05
Total to scrub:   536870912
Bytes scrubbed:   536870912  (100.00%)
Error summary:    csum=2 read=1
`

	parseScrubStatus(&fs, output)
	require.Equal(t, ScrubStateFinished, fs.ScrubState)
	require.Equal(t, int64(536870912), fs.ScrubTotalBytes)
	require.Equal(t, int64(2), fs.ScrubCsumErrors)
	require.Equal(t, int64(1), fs.ScrubReadErrors)
	require.NotNil(t, fs.ScrubStartedAt)
	require.NotNil(t, fs.ScrubFinishedAt)
}

func TestParseScrubStatusSynologyDSM7Finished(t *testing.T) {
	fs := Filesystem{}
	output := `scrub status for 9e14872a-781a-44e8-8983-6d1699dac7bd
        scrub started at Thu May  7 02:00:01 2026 and finished after 00:02:17
        data_extents_scrubbed: 1401095
        tree_extents_scrubbed: 119344
        data_bytes_scrubbed: 57018515456
        tree_bytes_scrubbed: 1955332096
        read_errors: 0
        csum_errors: 0
        verify_errors: 0
        no_csum: 9385705
        csum_discards: 0
        super_errors: 0
        malloc_errors: 0
        uncorrectable_errors: 0
        unverified_errors: 0
        corrected_errors: 0
        last_physical: 240555917312
`

	parseScrubStatus(&fs, output)
	require.Equal(t, "9e14872a-781a-44e8-8983-6d1699dac7bd", fs.UUID)
	require.Equal(t, ScrubStateFinished, fs.ScrubState)
	require.Equal(t, "00:02:17", fs.ScrubDuration)
	require.Equal(t, int64(58973847552), fs.ScrubScrubbedBytes)
	require.Equal(t, int64(58973847552), fs.ScrubTotalBytes)
	require.Equal(t, "no errors found", fs.ScrubErrorSummary)
	require.NotNil(t, fs.ScrubStartedAt)
	require.NotNil(t, fs.ScrubFinishedAt)
}

func TestParseScrubStatusSynologyDSM7NoStats(t *testing.T) {
	fs := Filesystem{}
	output := `scrub status for 8625a86f-fe31-4d6c-aa99-4e1c9b550fae
        no stats available
        data_extents_scrubbed: 0
        tree_extents_scrubbed: 0
        data_bytes_scrubbed: 0
        tree_bytes_scrubbed: 0
        read_errors: 0
        csum_errors: 0
        verify_errors: 0
        no_csum: 0
        csum_discards: 0
        super_errors: 0
        malloc_errors: 0
        uncorrectable_errors: 0
        unverified_errors: 0
        corrected_errors: 0
        last_physical: 0
`

	parseScrubStatus(&fs, output)
	require.Equal(t, "8625a86f-fe31-4d6c-aa99-4e1c9b550fae", fs.UUID)
	require.Equal(t, ScrubStateIdle, fs.ScrubState)
	require.Equal(t, int64(0), fs.ScrubScrubbedBytes)
	require.Equal(t, int64(0), fs.ScrubTotalBytes)
	require.Equal(t, "no errors found", fs.ScrubErrorSummary)
}

func TestDetectStartEnumeratesMountedFilesystems(t *testing.T) {
	mounts := []byte(`/dev/sda1 / btrfs rw 0 0
/dev/sda1 /home btrfs rw 0 0
/dev/sdb1 /data ext4 rw 0 0
`)

	commandOutputs := map[string][]byte{
		"btrfs filesystem show --raw /": []byte(`Label: 'tank'  uuid: 11111111-2222-3333-4444-555555555555
	Total devices 2 FS bytes used 512.00MiB
	devid    1 size 1073741824 used 805306368 path /dev/sda1
	devid    2 size 1073741824 used 268435456 path /dev/sdb1
`),
		"btrfs filesystem usage --raw /": []byte(`Overall:
    Device size:                   2147483648
    Device allocated:              1073741824
    Device unallocated:            1073741824
    Device missing:                0
    Used:                          805306368
    Free (estimated):              1342177280      (min: 1342177280)
    Free (statfs, df):             1207959552
    Data ratio:                    1.00
    Metadata ratio:                2.00
    Multiple profiles:             no
Data, single: total=536870912, used=268435456
Metadata, DUP: total=268435456, used=134217728
System, DUP: total=33554432, used=16384
`),
		"btrfs device stats /": []byte(`[/dev/sda1].write_io_errs   0
[/dev/sda1].read_io_errs    0
[/dev/sda1].flush_io_errs   0
[/dev/sda1].corruption_errs 0
[/dev/sda1].generation_errs 0
[/dev/sdb1].write_io_errs   0
[/dev/sdb1].read_io_errs    0
[/dev/sdb1].flush_io_errs   0
[/dev/sdb1].corruption_errs 0
[/dev/sdb1].generation_errs 0
`),
		"btrfs scrub status --raw /": []byte(`UUID:             11111111-2222-3333-4444-555555555555
Scrub started:    Wed Apr 10 12:34:56 2024
Status:           running
Duration:         0:00:05
Total to scrub:   536870912
Bytes scrubbed:   268435456  (50.00%)
Error summary:    no errors found
`),
	}

	detector := Detect{
		Logger: logrus.NewEntry(logrus.New()),
		ReadMountsFile: func(string) ([]byte, error) {
			return mounts, nil
		},
		LookPath: func(string) (string, error) {
			return "/usr/bin/btrfs", nil
		},
		RunCommand: func(name string, args ...string) ([]byte, error) {
			key := name + " " + joinArgs(args)
			output, ok := commandOutputs[key]
			if !ok {
				return nil, errors.New("unexpected command: " + key)
			}
			return output, nil
		},
	}

	filesystems, err := detector.Start()
	require.NoError(t, err)
	require.Len(t, filesystems, 1)
	require.Equal(t, "11111111-2222-3333-4444-555555555555", filesystems[0].UUID)
	require.Equal(t, "/", filesystems[0].MountPoint)
	require.Equal(t, FilesystemStatusOnline, filesystems[0].Status)
	require.Equal(t, 2, filesystems[0].DeviceCount)
	require.Len(t, filesystems[0].Devices, 2)
	require.Equal(t, 1, filesystems[0].Devices[0].ID)
}

func TestDetectMarksDegradedWhenDeviceMissing(t *testing.T) {
	mounts := []byte(`/dev/sda1 / btrfs rw 0 0
`)

	commandOutputs := map[string][]byte{
		"btrfs filesystem show --raw /": []byte(`Label: none  uuid: 99999999-2222-3333-4444-555555555555
	Total devices 2 FS bytes used 512.00MiB
	devid    1 size 1073741824 used 805306368 path /dev/sda1
	devid    2 size 1073741824 used 0 path missing
`),
		"btrfs filesystem usage --raw /": []byte(`Overall:
    Device size:                   2147483648
    Device allocated:              1073741824
    Device unallocated:            1073741824
    Device missing:                1073741824
    Used:                          805306368
    Free (estimated):              1342177280      (min: 1342177280)
    Free (statfs, df):             1207959552
    Data ratio:                    1.00
    Metadata ratio:                2.00
    Multiple profiles:             yes
Data, RAID1: total=536870912, used=268435456
Metadata, RAID1: total=268435456, used=134217728
System, RAID1: total=33554432, used=16384
`),
		"btrfs device stats /": []byte(`[/dev/sda1].write_io_errs   0`),
		"btrfs scrub status --raw /": []byte(`UUID:             99999999-2222-3333-4444-555555555555
Status:           not running
Error summary:    no errors found
`),
	}

	detector := Detect{
		Logger: logrus.NewEntry(logrus.New()),
		ReadMountsFile: func(string) ([]byte, error) {
			return mounts, nil
		},
		LookPath: func(string) (string, error) {
			return "/usr/bin/btrfs", nil
		},
		RunCommand: func(name string, args ...string) ([]byte, error) {
			key := name + " " + joinArgs(args)
			output, ok := commandOutputs[key]
			if !ok {
				return nil, errors.New("unexpected command: " + key)
			}
			return output, nil
		},
	}

	filesystems, err := detector.Start()
	require.NoError(t, err)
	require.Len(t, filesystems, 1)
	require.Equal(t, FilesystemStatusDegraded, filesystems[0].Status)
	require.True(t, filesystems[0].Devices[1].Missing)
}

func TestDetectReconcilesMountedSingleDeviceReportedMissing(t *testing.T) {
	mounts := []byte(`/dev/vg1/volume_1 /volume1 btrfs rw 0 0
`)

	commandOutputs := map[string][]byte{
		"btrfs filesystem show --raw /volume1": []byte(`Label: '2024.11.02-12:11:27 v72806'  uuid: 9e14872a-781a-44e8-8983-6d1699dac7bd
	Total devices 1 FS bytes used 63887634432
	devid    1 size 0 used 0 path missing
`),
		"btrfs filesystem usage --raw /volume1": []byte(`Overall:
    Device size:                   468200000000
    Device allocated:              92274688000
    Device unallocated:            375925312000
    Device missing:                468200000000
    Used:                          59500000000
    Free (estimated):              408700000000      (min: 408700000000)
    Free (statfs, df):             408700000000
    Data ratio:                    1.00
    Metadata ratio:                2.00
    Multiple profiles:             no
Data, single: total=68719476736, used=59500000000
Metadata, DUP: total=17179869184, used=4294967296
System, DUP: total=33554432, used=16384
`),
		"btrfs device stats /volume1": []byte(`[/dev/vg1/volume_1].write_io_errs   0
[/dev/vg1/volume_1].read_io_errs    0
[/dev/vg1/volume_1].flush_io_errs   0
[/dev/vg1/volume_1].corruption_errs 0
[/dev/vg1/volume_1].generation_errs 0
`),
		"btrfs scrub status --raw /volume1": []byte(`UUID:             9e14872a-781a-44e8-8983-6d1699dac7bd
Status:           not running
Error summary:    no errors found
`),
	}

	detector := Detect{
		Logger: logrus.NewEntry(logrus.New()),
		ReadMountsFile: func(string) ([]byte, error) {
			return mounts, nil
		},
		LookPath: func(string) (string, error) {
			return "/usr/bin/btrfs", nil
		},
		RunCommand: func(name string, args ...string) ([]byte, error) {
			key := name + " " + joinArgs(args)
			output, ok := commandOutputs[key]
			if !ok {
				return nil, errors.New("unexpected command: " + key)
			}
			return output, nil
		},
	}

	filesystems, err := detector.Start()
	require.NoError(t, err)
	require.Len(t, filesystems, 1)
	require.Equal(t, FilesystemStatusOnline, filesystems[0].Status)
	require.Equal(t, int64(0), filesystems[0].DeviceMissing)
	require.Len(t, filesystems[0].Devices, 1)
	require.False(t, filesystems[0].Devices[0].Missing)
	require.Equal(t, "/dev/vg1/volume_1", filesystems[0].Devices[0].Path)
	require.Equal(t, int64(468200000000), filesystems[0].Devices[0].Size)
}

func TestDetectPrefersHostProcMountsWhenAvailable(t *testing.T) {
	hostMounts := []byte(`/dev/vg1/volume_1 / btrfs rw 0 0
/dev/vg1/volume_1 /etc/hosts btrfs rw 0 0
/dev/vg1/volume_1 /opt/scrutiny/config btrfs rw 0 0
/dev/vg1/volume_1 /volume1 btrfs rw 0 0
/dev/vg2/volume_2 /volume2 btrfs rw 0 0
`)
	containerMounts := []byte(`/dev/mapper/vg1-volume_1 / btrfs rw 0 0
`)

	commandOutputs := map[string][]byte{
		"btrfs filesystem show --raw /volume1": []byte(`Label: 'vol1'  uuid: 9e14872a-781a-44e8-8983-6d1699dac7bd
	Total devices 1 FS bytes used 59298209792
	devid    1 size 468151435264 used 240543334400 path /dev/mapper/vg1-volume_1
`),
		"btrfs filesystem usage --raw /volume1": []byte(`Overall:
    Device size:                      468151435264
    Device allocated:                 240543334400
    Device unallocated:               227608100864
    Device missing:                              0
    Used:                              60292476928
    Free (estimated):                 392650477568      (min: 278846427136)
    Data ratio:                               1.00
    Metadata ratio:                           2.00
Data,single: Size:223346688000, Used:58304311296
Metadata,DUP: Size:8589934592, Used:994033664
System,DUP: Size:8388608, Used:49152
`),
		"btrfs device stats /volume1":     []byte(`[/dev/mapper/vg1-volume_1].write_io_errs   0`),
		"btrfs scrub status --raw /volume1": []byte(`scrub status for 9e14872a-781a-44e8-8983-6d1699dac7bd
        no stats available
`),
		"btrfs filesystem show --raw /volume2": []byte(`Label: 'vol2'  uuid: 8625a86f-fe31-4d6c-aa99-4e1c9b550fae
	Total devices 1 FS bytes used 39512166400
	devid    1 size 314606354432 used 114915540992 path /dev/mapper/vg2-volume_2
`),
		"btrfs filesystem usage --raw /volume2": []byte(`Overall:
    Device size:                      314606354432
    Device allocated:                 114915540992
    Device unallocated:               199690813440
    Device missing:                              0
    Used:                              39556878336
    Free (estimated):                 272974639104      (min: 173129232384)
    Data ratio:                               1.00
    Metadata ratio:                           2.00
Data,single: Size:112751280128, Used:39467454464
Metadata,DUP: Size:1073741824, Used:44695552
System,DUP: Size:8388608, Used:16384
`),
		"btrfs device stats /volume2":     []byte(`[/dev/mapper/vg2-volume_2].write_io_errs   0`),
		"btrfs scrub status --raw /volume2": []byte(`scrub status for 8625a86f-fe31-4d6c-aa99-4e1c9b550fae
        no stats available
`),
	}

	detector := Detect{
		Logger: logrus.NewEntry(logrus.New()),
		ReadMountsFile: func(path string) ([]byte, error) {
			switch path {
			case "/host/proc/mounts":
				return hostMounts, nil
			case "/proc/mounts":
				return containerMounts, nil
			default:
				return nil, os.ErrNotExist
			}
		},
		LookPath: func(string) (string, error) {
			return "/usr/bin/btrfs", nil
		},
		RunCommand: func(name string, args ...string) ([]byte, error) {
			key := name + " " + joinArgs(args)
			output, ok := commandOutputs[key]
			if !ok {
				return nil, errors.New("unexpected command: " + key)
			}
			return output, nil
		},
	}

	filesystems, err := detector.Start()
	require.NoError(t, err)
	require.Len(t, filesystems, 2)
	require.Equal(t, "/volume1", filesystems[0].MountPoint)
	require.Equal(t, "9e14872a-781a-44e8-8983-6d1699dac7bd", filesystems[0].UUID)
	require.Equal(t, "/volume2", filesystems[1].MountPoint)
	require.Equal(t, "8625a86f-fe31-4d6c-aa99-4e1c9b550fae", filesystems[1].UUID)
}

func TestDetectClearsFalseDeviceMissingForMountedSingleDevice(t *testing.T) {
	mounts := []byte(`/dev/vg2/volume_2 /volume2 btrfs rw 0 0
`)

	commandOutputs := map[string][]byte{
		"btrfs filesystem show --raw /volume2": []byte(`Label: 'vol2'  uuid: 8625a86f-fe31-4d6c-aa99-4e1c9b550fae
	Total devices 1 FS bytes used 39512166400
	devid    1 size 314606354432 used 114915540992 path /dev/mapper/vg2-volume_2
`),
		"btrfs filesystem usage --raw /volume2": []byte(`Overall:
    Device size:                      314606354432
    Device allocated:                 114915540992
    Device unallocated:               199690813440
    Device missing:                   314606354432
    Used:                              39556878336
    Free (estimated):                 272974639104      (min: 173129232384)
    Data ratio:                               1.00
    Metadata ratio:                           2.00
Data,single: Size:112751280128, Used:39467454464
Metadata,DUP: Size:1073741824, Used:44695552
System,DUP: Size:8388608, Used:16384
`),
		"btrfs device stats /volume2": []byte(`[/dev/mapper/vg2-volume_2].write_io_errs   0`),
		"btrfs scrub status --raw /volume2": []byte(`scrub status for 8625a86f-fe31-4d6c-aa99-4e1c9b550fae
        no stats available
`),
	}

	detector := Detect{
		Logger: logrus.NewEntry(logrus.New()),
		ReadMountsFile: func(string) ([]byte, error) {
			return mounts, nil
		},
		LookPath: func(string) (string, error) {
			return "/usr/bin/btrfs", nil
		},
		RunCommand: func(name string, args ...string) ([]byte, error) {
			key := name + " " + joinArgs(args)
			output, ok := commandOutputs[key]
			if !ok {
				return nil, errors.New("unexpected command: " + key)
			}
			return output, nil
		},
	}

	filesystems, err := detector.Start()
	require.NoError(t, err)
	require.Len(t, filesystems, 1)
	require.Equal(t, FilesystemStatusOnline, filesystems[0].Status)
	require.Equal(t, "/volume2", filesystems[0].MountPoint)
	require.Equal(t, int64(0), filesystems[0].DeviceMissing)
	require.Len(t, filesystems[0].Devices, 1)
	require.False(t, filesystems[0].Devices[0].Missing)
	require.Equal(t, "/dev/vg2/volume_2", filesystems[0].Devices[0].Path)
}

func TestParseBtrfsTime(t *testing.T) {
	ts, err := parseBtrfsTime("Wed Apr 10 12:34:56 2024")
	require.NoError(t, err)
	require.Equal(t, time.Date(2024, time.April, 10, 12, 34, 56, 0, time.UTC), ts.UTC())
}

func joinArgs(args []string) string {
	return strings.Join(args, " ")
}
