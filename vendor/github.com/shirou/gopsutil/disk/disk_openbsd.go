// +build openbsd

package disk

import (
	"bytes"
	"encoding/binary"
	"path"
	"syscall"
	"unsafe"

	"github.com/shirou/gopsutil/internal/common"
)

func Partitions(all bool) ([]PartitionStat, error) {
	var ret []PartitionStat

	// get length
	count, err := syscall.Getfsstat(nil, MNT_WAIT)
	if err != nil {
		return ret, err
	}

	fs := make([]Statfs, count)
	_, err = Getfsstat(fs, MNT_WAIT)

	for _, stat := range fs {
		opts := "rw"
		if stat.F_flags&MNT_RDONLY != 0 {
			opts = "ro"
		}
		if stat.F_flags&MNT_SYNCHRONOUS != 0 {
			opts += ",sync"
		}
		if stat.F_flags&MNT_NOEXEC != 0 {
			opts += ",noexec"
		}
		if stat.F_flags&MNT_NOSUID != 0 {
			opts += ",nosuid"
		}
		if stat.F_flags&MNT_NODEV != 0 {
			opts += ",nodev"
		}
		if stat.F_flags&MNT_ASYNC != 0 {
			opts += ",async"
		}

		d := PartitionStat{
			Device:     common.IntToString(stat.F_mntfromname[:]),
			Mountpoint: common.IntToString(stat.F_mntonname[:]),
			Fstype:     common.IntToString(stat.F_fstypename[:]),
			Opts:       opts,
		}
		if all == false {
			if !path.IsAbs(d.Device) || !common.PathExists(d.Device) {
				continue
			}
		}

		ret = append(ret, d)
	}

	return ret, nil
}

func IOCounters() (map[string]IOCountersStat, error) {
	ret := make(map[string]IOCountersStat)

	r, err := syscall.Sysctl("hw.diskstats")
	if err != nil {
		return nil, err
	}
	buf := []byte(r)
	length := len(buf)

	count := int(uint64(length) / uint64(sizeOfDiskstats))

	// parse buf to Diskstats
	for i := 0; i < count; i++ {
		b := buf[i*sizeOfDiskstats : i*sizeOfDiskstats+sizeOfDiskstats]
		d, err := parseDiskstats(b)
		if err != nil {
			continue
		}
		name := common.IntToString(d.Name[:])

		ds := IOCountersStat{
			ReadCount:  d.Rxfer,
			WriteCount: d.Wxfer,
			ReadBytes:  d.Rbytes,
			WriteBytes: d.Wbytes,
			Name:       name,
		}
		ret[name] = ds
	}

	return ret, nil
}

// BT2LD(time)     ((long double)(time).sec + (time).frac * BINTIME_SCALE)

// Getfsstat is borrowed from pkg/syscall/syscall_freebsd.go
// change Statfs_t to Statfs in order to get more information
func Getfsstat(buf []Statfs, flags int) (n int, err error) {
	var _p0 unsafe.Pointer
	var bufsize uintptr
	if len(buf) > 0 {
		_p0 = unsafe.Pointer(&buf[0])
		bufsize = unsafe.Sizeof(Statfs{}) * uintptr(len(buf))
	}
	r0, _, e1 := syscall.Syscall(syscall.SYS_GETFSSTAT, uintptr(_p0), bufsize, uintptr(flags))
	n = int(r0)
	if e1 != 0 {
		err = e1
	}
	return
}

func parseDiskstats(buf []byte) (Diskstats, error) {
	var ds Diskstats
	br := bytes.NewReader(buf)
	//	err := binary.Read(br, binary.LittleEndian, &ds)
	err := common.Read(br, binary.LittleEndian, &ds)
	if err != nil {
		return ds, err
	}

	return ds, nil
}

func Usage(path string) (*UsageStat, error) {
	stat := syscall.Statfs_t{}
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return nil, err
	}
	bsize := stat.F_bsize

	ret := &UsageStat{
		Path:        path,
		Fstype:      getFsType(stat),
		Total:       (uint64(stat.F_blocks) * uint64(bsize)),
		Free:        (uint64(stat.F_bavail) * uint64(bsize)),
		InodesTotal: (uint64(stat.F_files)),
		InodesFree:  (uint64(stat.F_ffree)),
	}

	ret.InodesUsed = (ret.InodesTotal - ret.InodesFree)
	ret.InodesUsedPercent = (float64(ret.InodesUsed) / float64(ret.InodesTotal)) * 100.0
	ret.Used = (uint64(stat.F_blocks) - uint64(stat.F_bfree)) * uint64(bsize)
	ret.UsedPercent = (float64(ret.Used) / float64(ret.Total)) * 100.0

	return ret, nil
}

func getFsType(stat syscall.Statfs_t) string {
	return common.IntToString(stat.F_fstypename[:])
}
