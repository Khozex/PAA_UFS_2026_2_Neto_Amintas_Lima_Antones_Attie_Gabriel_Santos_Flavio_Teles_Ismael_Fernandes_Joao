package hardware

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

type Info struct {
	OS     string
	Arch   string
	Go     string
	CPU    string
	Cores  int
	RAMGiB string
}

func Detect() Info {
	return Info{
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		Go:     runtime.Version(),
		CPU:    cpuModel(),
		Cores:  runtime.NumCPU(),
		RAMGiB: totalRAM(),
	}
}

func cpuModel() string {
	return procField("/proc/cpuinfo", "model name")
}

func totalRAM() string {
	kb := procField("/proc/meminfo", "MemTotal")
	fields := strings.Fields(kb)
	if len(fields) == 0 {
		return "desconhecido"
	}
	n, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "desconhecido"
	}
	return fmt.Sprintf("%.1f GiB", n/1024/1024)
}

func procField(path, key string) string {
	file, err := os.Open(path)
	if err != nil {
		return "desconhecido"
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		name, value, ok := strings.Cut(scanner.Text(), ":")
		if ok && strings.TrimSpace(name) == key {
			return strings.TrimSpace(value)
		}
	}
	return "desconhecido"
}
