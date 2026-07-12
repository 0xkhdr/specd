package verify

import "fmt"

// Limits is adapter-neutral verify resource policy. Zero leaves a dimension
// unset. Production callers supply non-zero values through DefaultLimits.
type Limits struct {
	CPUSeconds  int
	MemoryBytes int64
	Processes   int
	OutputBytes int
	WallSeconds int
	FileBytes   int64
}

var DefaultLimits = Limits{
	CPUSeconds: 300, MemoryBytes: 2 << 30, Processes: 256,
	OutputBytes: 8 << 20, WallSeconds: 600, FileBytes: 1 << 30,
}

func (l Limits) shellPrefix() string {
	parts := ""
	if l.CPUSeconds > 0 {
		parts += fmt.Sprintf("ulimit -t %d; ", l.CPUSeconds)
	}
	if l.MemoryBytes > 0 {
		parts += fmt.Sprintf("ulimit -v %d; ", (l.MemoryBytes+1023)/1024)
	}
	if l.Processes > 0 {
		parts += fmt.Sprintf("ulimit -u %d; ", l.Processes)
	}
	if l.FileBytes > 0 {
		parts += fmt.Sprintf("ulimit -f %d; ", (l.FileBytes+511)/512)
	}
	return parts
}
