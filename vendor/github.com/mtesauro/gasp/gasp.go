// Package Gasp is a library for creating AppSec Pipelines in Golang
package gasp

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"

	"gopkg.in/yaml.v2"
)

// Interface for checking for prerequisites
type Prereq interface {
	VerifyPrereqs(d Deps)
}

type Deps struct {
	Bins          []string // Required binaries that need to be in $PATH
	Files         []string // Required files to run Gasp
	FilePath      string   // Path to required files
	ExternalFiles []string // Full path to external files (non-GASP) required by an implementation
}

// Interface for reading configs
type Config interface {
	ReadMaster(mstr *M, c ConfigOpts) // Read main configuration
	ReadSecPipe(sec *S, c ConfigOpts) // Read tools configuration
}

type ConfigOpts struct {
	Ctype string // type of config store - currently only "Local Files" supported
	Path  string // path were configs are stored
}

// Interface for listing images
type Images interface {
	SyncImages(sec *S) // Ensure needed images are available in the image repo
}

// Interface for handling event imput (command-line or otherwise)
type Event interface {
	ReadArgs(a *[]string, evArgs *EventArgs)
	GetId()
	Startup()
	Pipeline()
	Final()
	Cleanup()
}

// Struct for event arguments
type EventArgs struct {
	Required    []string          // names of required fields will be in this slice of strings
	Profile     string            // Required - named pipeline from master.yaml, appsec.pipeline or [app name]-pipeline.yaml
	Dir         string            // default to empty string - local directory to copy to Docke shared volume
	DryRun      bool              // default = false - Do this pipeline run without launching actual dockers, etc
	Clean       bool              // default = false - Remove the containers and volumes once completed
	Vol         string            // default = "" - Specify the name of the results volumne to use
	AppName     string            // default = "" - Name of the app being tested
	EventType   string            // default = "Command-line" - Type of event that caused the pipeline run
	AppProfile  string            // default = "" - App specific pipeline profile to run sent in [app-name]-pipeline.yaml
	AppToolProf string            // default = "" - App specific tool profile to use during pipeline run from [app-name]-tool.yaml
	Target      string            // default = "" - Docker name of the target for the pipeline run - to be launched before run
	PipeType    string            // Required - {"static", "dynamic"}
	Loc         string            // default = "" - Location of source code for static test runs
	ToolConf    map[string]string // Required - Parameters needed to run each tool in the named pipeline
	DojoHost    string            // Required - host name of the Dojo instance to push the run restults to
	DojoApiKey  string            // Required - API key to talk to Dojo's REST API
	DojoProdId  string            // Required - The Product ID from Dojo to submit the results for this test run
	//DojoNewEng boot // default = true - Create a new engagement for each pipeline run?
}

// Sturct for master.yaml aka main configuration
type M struct {
	Version string              `yaml:"version"`
	Global  Gconf               `yaml:"global"`
	Prof    map[string]Profiles `yaml:"profiles"`
}

type Gconf struct {
	MinSev string `yaml:"min-severity"`
}

type Profiles struct {
	Pipeline []Tools
	Startup  []Tools
	RunEvery []Tools
	Final    []Tools
}

type Tools struct {
	Tool    string `yaml:"tool"`
	Options string `yaml:"options"`
}

// Struct for secpipeline-config.yaml aka tools configuration
type S struct {
	Version string             `yaml:"version"`
	T       map[string]SecTool `yaml:"tools"`
}

type SecTool struct {
	Version       string            `yaml:"version"`
	Tags          []string          `yaml:"tags"`
	TType         string            `yaml:"type"`
	ScanType      string            `yaml:"scan_type"`
	IconSm        string            `yaml:"icon-sm"`
	IconLg        string            `yaml:"icon-lg"`
	Description   string            `yaml:"description"`
	Url           string            `yaml:"url"`
	Documentation string            `yaml:"documentation"`
	Docker        string            `yaml:"docker"`
	Parameters    map[string]PMeta  `yaml:"parameters"`
	Cmds          map[string]string `yaml:"commands"`
	Pfls          map[string]string `yaml:"profiles"`
}

type PMeta struct {
	PType    string `yaml:"type"`
	DataType string `yaml:"data_type"`
	Desc     string `yaml:"description"`
}

// Default types for running Gasp locally aka laptop runs

// Check dependencies required for the implemented version of Gasp
type LocalDeps struct {
}

func (ld *LocalDeps) VerifyPrereqs(d Deps) {
	ready := true

	// Check for required binaries
	if len(d.Bins) > 0 {
		for k := range d.Bins {
			_, err := exec.LookPath(d.Bins[k])
			if err != nil {
				fmt.Printf("ERROR: The %s command must be installed and in your path.\n", d.Bins[k])
				fmt.Println("\tPlease install it so you can run Gasp")
				ready = false
			}
		}
	}

	// Check for required files
	if len(d.Files) > 0 {
		for k := range d.Files {
			fp := path.Join(d.FilePath, d.Files[k])
			_, err := os.Stat(fp)
			if err != nil {
				fmt.Printf("ERROR: The %s file must be exist in %s.\n", d.Files[k], d.FilePath)
				fmt.Println("\tPlease add that file so you can run Gasp")
				ready = false
			}
		}
	}

	// Check for required files
	if len(d.ExternalFiles) > 0 {
		for k := range d.ExternalFiles {
			_, err := os.Stat(d.ExternalFiles[k])
			if err != nil {
				fmt.Printf("ERROR: The %s file must exist and was not found\n", d.ExternalFiles[k])
				fmt.Println("\tPlease add that file so you can run Gasp")
				ready = false
			}
		}
	}

	if !ready {
		log.Fatal("Prerequisites to run this program missing, please correct errors above")
	}
}

// Read configuration yaml files from local disk
type LocalConfigs struct {
	ConfFile string
	ToolFile string
}

func (l *LocalConfigs) ReadMaster(mstr *M, c ConfigOpts) {
	Info.Println("Reading master.yaml file")

	fpath := path.Join(c.Path, l.ConfFile)

	f, err := ioutil.ReadFile(fpath)
	if err != nil {
		fmt.Println("Problem reading master.yaml from ", fpath)
		fmt.Printf("Error is: %s", err)
	}

	err = yaml.Unmarshal(f, mstr)
	if err != nil {
		fmt.Println("error: %v", err)
	}
}

func (l *LocalConfigs) ReadSecPipe(sec *S, c ConfigOpts) {
	fmt.Println("Reading secpipeline-conf.yaml")

	fpath := path.Join(c.Path, l.ToolFile)

	f, err := ioutil.ReadFile(fpath)
	if err != nil {
		fmt.Println("Problem reading secpipeline-conf.yaml from ", fpath)
		fmt.Printf("Error is: %s", err)
	}

	err = yaml.Unmarshal(f, sec)
	if err != nil {
		fmt.Println("error: %v", err)
	}
}

// Logging items
var (
	Trace   *log.Logger
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
)

func InitLogs(traceHandle io.Writer, infoHandle io.Writer, warningHandle io.Writer, errorHandle io.Writer) map[string]*log.Logger {

	Trace = log.New(traceHandle, "TRACE:   ", log.Ldate|log.Ltime|log.Lshortfile)
	Info = log.New(infoHandle, "INFO:    ", log.Ldate|log.Ltime)
	Warning = log.New(warningHandle, "WARNING: ", log.Ldate|log.Ltime)
	Error = log.New(errorHandle, "ERROR:   ", log.Ldate|log.Ltime|log.Lshortfile)

	// Allows for these to be use globally
	// Trace.Println("Trace message")
	// Info.Println("Info message")
	// Warning.Println("Warning message")
	// Error.Println("Error message")

	return map[string]*log.Logger{
		"trace": Trace,
		"info":  Info,
		"warn":  Warning,
		"error": Error,
	}
}

func SetupLogging(name string, logPath string, timestamp bool) map[string]*log.Logger {
	// Setup logging
	when := ""
	if timestamp {
		n := time.Now()
		when = "_" + strconv.Itoa(int(n.UnixNano()))
	}

	logName := name + when + ".log"
	fullPath := path.Join(logPath, logName)
	logFile, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("\nPlease create any directories needed to write logs to %v\n\n", logPath)
		log.Fatalf("Failed to open log file %s.  Error was:\n  %+v\n", logPath, err)
	}
	// Log everthing to the specificied log file location
	return InitLogs(logFile, logFile, logFile, logFile)
}
