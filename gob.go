package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	fswatch "github.com/andreaskoch/go-fswatch"
)

type Gob struct {
	Cmd    *exec.Cmd
	Config *Config

	// The full source path of the file to build
	InputPath string

	// The absolute path of the user input
	Dir string

	// The "package" path name of the program we're building
	PackagePath string

	// The file name of the user input
	Filename string

	// The path to the binary file (e.g. output of build)
	Binary string
}

// Builder configuration options
type Config struct {
	GoPath   string
	BuildDir string
	SrcDir   string

	Stdout io.Writer
	Stderr io.Writer
}

func NewGob() *Gob {
	return &Gob{
		Config: DefaultConfig(),
	}
}

func DefaultConfig() *Config {
	goPath := os.Getenv("GOPATH")

	return &Config{
		GoPath:   goPath,
		BuildDir: goPath + "/gob/build",
		SrcDir:   goPath + "/src/",
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
	}
}

// Simple print method to prefix all output with "[gob]"
func (g *Gob) Print(msg string) {
	fmt.Println("[gob] " + msg)
}

// Simple argument validation
// to short circuit compilation errors
func (g *Gob) IsValidSrc() bool {
	var err error

	// Make sure the user provided enough args to cmd line
	if len(os.Args) < 2 {
		g.Print("Please provide a valid source file to run.")
		return false
	}

	// Users raw input
	g.InputPath = os.Args[1]

	// Stores the absolute path of our file or package
	// Used to check to see if the package/file exists from root
	var absPath string

	// Handle filename and "package" as inputs
	g.Dir, g.Filename = filepath.Split(g.InputPath)
	if strings.Contains(g.Filename, ".") {
		g.Dir, err = filepath.Abs(g.Dir)

		g.PackagePath, err = filepath.Rel(g.Config.SrcDir, g.Dir)
		if err != nil {
			fmt.Println(err)
			return false
		}

		absPath = filepath.Join(g.Config.SrcDir, g.PackagePath, g.Filename)
	} else {
		g.PackagePath = g.InputPath
		absPath = filepath.Join(g.Config.SrcDir, g.PackagePath)
	}

	// Save our "tmp" binary as the last element of the input
	g.Binary = g.Config.BuildDir + "/" + filepath.Base(g.Dir)

	// Make sure the file/package we're trying to build exists
	_, err = os.Stat(absPath)
	if os.IsNotExist(err) {
		g.Print("Please provide a valid source file/package to run.")
		return false
	}

	return true
}

// Create a temp directory based on the configuration.
// This is where all the binaries are stored
func (g *Gob) Setup() {
	_, err := os.Stat(g.Config.BuildDir)
	if os.IsNotExist(err) {
		g.Print("creating temporary build directory... " + g.Config.BuildDir)
		os.MkdirAll(g.Config.BuildDir, 0777)
	}
}

// Build the source and save the binary
func (g *Gob) Build() bool {
	cmd := exec.Command("go", "build", "-o", g.Binary, g.InputPath)
	cmd.Stdout = g.Config.Stdout
	cmd.Stderr = g.Config.Stderr

	g.Print("building src... " + g.InputPath)
	if err := cmd.Run(); err != nil {
		fmt.Println(err)
		cmd.Process.Kill()
		return false
	}

	return true
}

// Run our programs binary
func (g *Gob) Run() {
	cmd := exec.Command(g.Binary)

	cmd.Stdout = g.Config.Stdout
	cmd.Stderr = g.Config.Stderr

	g.Print("starting application...")
	if err := cmd.Start(); err != nil {
		fmt.Println(err)
		cmd.Process.Kill()
	}

	g.Cmd = cmd
}

// Watch the filesystem for any changes
// and restart the application if detected
func (g *Gob) Watch() {
	skipNoFile := func(path string) bool {
		return false
	}

	folder := fswatch.NewFolderWatcher(filepath.Join(g.Config.SrcDir), true, skipNoFile).Start()

	for folder.IsRunning() {
		select {
		case <-folder.Change:
			g.Print("restarting application...")
			g.Cmd.Process.Kill()
			build := g.Build()
			if build {
				g.Run()
			}
		}
	}
}

func main() {
	gob := NewGob()
	if !gob.IsValidSrc() {
		return
	}

	gob.Print("initializing program...")

	gob.Setup()
	build := gob.Build()
	if build {
		gob.Run()
	}
	gob.Watch()
}
