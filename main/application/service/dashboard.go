package service

import (
	"bytes"
	"errors"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"moss/application/dto"
	"moss/infrastructure/general/constant"

	"github.com/google/pprof/profile"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

// SystemLoadPercent 系统平均负载
func SystemLoadPercent() float64 {
	info, _ := load.Avg()
	if info == nil {
		return -1
	}
	return (info.Load1 + info.Load5 + info.Load15) / 3
}

// SystemCPUPercent 系统CPU使用率
func SystemCPUPercent(interval time.Duration) (_ float64, err error) {
	v, err := cpu.Percent(interval, false)
	if err != nil {
		return
	}
	return v[0], nil
}

// SystemMemoryPercent 系统内存使用率
func SystemMemoryPercent() (_ float64, err error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return
	}
	return v.UsedPercent, nil
}

// SystemDiskPercents 系统硬盘占用率
func SystemDiskPercents() (res []dto.DiskInfo, err error) {
	parts, err := disk.Partitions(false)
	if err != nil {
		return
	}

	for _, part := range parts {
		// 先过滤空挂载点
		if part.Mountpoint == "" {
			continue
		}

		// 过滤系统保留分区和特殊文件系统
		if !isPrimaryPartition(part) {
			continue
		}

		diskInfo, err := disk.Usage(part.Mountpoint)
		if err != nil {
			continue
		}

		// 过滤无效分区：容量为0
		if diskInfo.Total == 0 {
			continue
		}

		res = append(res, dto.DiskInfo{
			Name:        part.Mountpoint,
			UsedPercent: diskInfo.UsedPercent,
		})
	}
	return
}

// isPrimaryPartition 判断是否为主要分区（过滤系统保留分区和特殊文件系统）
func isPrimaryPartition(part disk.PartitionStat) bool {
	// 过滤掉空挂载点
	if part.Mountpoint == "" {
		return false
	}

	// 过滤掉特殊文件系统
	excludedFstypes := []string{"proc", "sysfs", "devtmpfs", "tmpfs", "cgroup", "overlay", "squashfs"}
	for _, fs := range excludedFstypes {
		if strings.EqualFold(part.Fstype, fs) {
			return false
		}
	}

	// Windows 下过滤掉隐藏分区和系统保留分区
	if runtime.GOOS == "windows" {
		// 只显示形如 "C:"、"D:" 的盘符
		if len(part.Mountpoint) != 2 || part.Mountpoint[1] != ':' {
			return false
		}
		// 过滤掉无效的盘符（A:、B: 通常用于软驱）
		if part.Mountpoint[0] < 'C' || part.Mountpoint[0] > 'Z' {
			return false
		}
	}

	// Linux 下过滤掉特殊挂载点
	if runtime.GOOS == "linux" {
		// 过滤掉 /proc、/sys、/dev 等特殊目录
		excludedMountpoints := []string{"/proc", "/sys", "/dev", "/run", "/boot/efi"}
		for _, mp := range excludedMountpoints {
			if strings.HasPrefix(part.Mountpoint, mp) {
				return false
			}
		}
	}

	return true
}

// AppCPUPercent 应用cpu使用率
func AppCPUPercent() (res float64, err error) {
	p, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return
	}
	if res, err = p.CPUPercent(); err != nil {
		return
	}
	if res > 100 {
		res = 100
	}
	return
}

// AppUsedMemory 应用内存占用（字节）
// 使用读取 profile 的方式统计内存占用，相对比较准确
func AppUsedMemory() (res uint64, err error) {
	// 获取内存 profile 数据
	memProfile := pprof.Lookup("heap")
	if memProfile == nil {
		return res, errors.New("heap profile not found")
	}
	buf := &bytes.Buffer{}
	if err = memProfile.WriteTo(buf, 0); err != nil {
		return
	}
	// 解析缓冲区中的内存 profile 数据
	prof, err := profile.Parse(buf)
	if err != nil {
		return
	}
	var memInuseSpace int64
	for _, sample := range prof.Sample {
		if len(sample.Value) >= 4 {
			memInuseSpace += sample.Value[3]
		}
	}
	return uint64(memInuseSpace), nil
}

func AppInfo() *dto.AppInfo {
	return &dto.AppInfo{
		Name:    constant.AppName,
		Version: constant.AppVersion,
	}
}
