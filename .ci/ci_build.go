package main

import (
	"fmt"
	"github.com/Depau/ttyc"
	"os"
	"os/exec"
	"runtime"
)

const JOBS = -1 // -1 => use number of usable CPUs

type PlatformInfo struct {
	OS       string
	Arch     string
	ExtraEnv []string
}

var CommonPlatforms = []PlatformInfo{
	{"linux", "386", []string{}},
	{"linux", "amd64", []string{}},
	{"linux", "arm", []string{}},
	{"linux", "arm64", []string{}},
	{"linux", "mips", []string{"GOMIPS=softfloat"}},
	{"linux", "mips64", []string{"GOMIPS=softfloat"}},
	{"linux", "mips64le", []string{"GOMIPS=softfloat"}},
	{"linux", "mipsle", []string{"GOMIPS=softfloat"}},
	{"linux", "ppc64", []string{}},
	{"linux", "ppc64le", []string{}},
	{"linux", "riscv64", []string{}},
	{"linux", "s390x", []string{}},
	{"android", "386", []string{}},
	{"android", "amd64", []string{}},
	//{"android", "arm", []string{}}, // CGO does not work
	{"android", "arm64", []string{}},
	{"darwin", "amd64", []string{}},
	{"darwin", "arm64", []string{}},
	{"freebsd", "386", []string{}},
	{"freebsd", "amd64", []string{}},
	{"freebsd", "arm", []string{}},
	{"freebsd", "arm64", []string{}},
	{"netbsd", "386", []string{}},
	{"netbsd", "amd64", []string{}},
	{"netbsd", "arm", []string{}},
	{"netbsd", "arm64", []string{}},
	{"openbsd", "386", []string{}},
	{"openbsd", "amd64", []string{}},
	{"openbsd", "arm", []string{}},
	{"openbsd", "arm64", []string{}},
	{"openbsd", "mips64", []string{"GOMIPS=softfloat"}},
}

var WisttyAdditionalPlatforms = []PlatformInfo{
	{"windows", "386", []string{}},
	{"windows", "amd64", []string{}},
	{"windows", "arm", []string{}},
	{"dragonfly", "amd64", []string{}},
}

func BuildPlatformExe(exe string, plat PlatformInfo, absoluteOutdir string, semaphore <-chan interface{}) {
	defer func() { <-semaphore }()

	fmt.Printf("Building %s for %s/%s\n", exe, plat.OS, plat.Arch)
	outname := absoluteOutdir + "/" + fmt.Sprintf("%s-%s-%s-%s", exe, ttyc.VERSION, plat.OS, plat.Arch)
	if plat.OS == "windows" {
		outname += ".exe"
	}
	command := exec.Command("go", "build", "-ldflags=-s -w", "-o", outname, "-trimpath")
	if runtime.GOOS == "linux" && runtime.GOOS == plat.OS && runtime.GOARCH == plat.Arch {
		command.Args = append(command.Args, "-ldflags", "-linkmode external -extldflags \"-fno-PIC -static\"")
	}

	command.Dir = "cmd/" + exe
	command.Env = os.Environ()
	command.Env = append(command.Env, "GOOS="+plat.OS, "GOARCH="+plat.Arch)
	command.Env = append(command.Env, plat.ExtraEnv...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = nil

	err := command.Run()
	if err != nil {
		fmt.Println("Build failed:", err)
	}
}

//goland:noinspection GoBoolExpressions // suppress check that jobs <= 0 is always true
func BuildExes(exe string, plats []PlatformInfo, absoluteOutdir string) {
	jobs := JOBS
	if jobs <= 0 {
		jobs = runtime.NumCPU()
	}
	fmt.Println("Running", jobs, "jobs in parallel")

	semaphore := make(chan interface{}, jobs)

	for _, plat := range plats {
		semaphore <- nil
		go BuildPlatformExe(exe, plat, absoluteOutdir, semaphore)
	}
}

func main() {
	path, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	BuildExes("ttyc", CommonPlatforms, path+"/build")
	BuildExes("wistty", append(CommonPlatforms, WisttyAdditionalPlatforms...), path+"/build")
}
