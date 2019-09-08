// gdocker
package gdocker

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	g "github.com/appsecpipeline/gasp"
	uuid "github.com/satori/go.uuid"
	//"github.com/spf13/viper"
)

// Interfaces from Gasp for this implementation

// Implementation of the Images interface from Gasp
type LocalDockers struct {
	images []Image
}

type Image struct {
	fullName string
	name     string
	tag      string
	id       string
	created  string
	size     string
}

func (ldock *LocalDockers) SyncImages(sec *g.S) {
	infoLog.Println("Ensuring needed tool images are available in the container repo")

	// Get a list of images available in the repo
	repoImages := listImages(ldock)

	// Diff needed images against available ones
	missing := diffImages(repoImages, sec)

	// Pull any needed images so all tools containers are available
	pullImages(missing)
	infoLog.Println("All needed tool images are available in the container repo")
}

// Implementation of the Event interface from gasp
type LocalEvent struct {
}

func (levent *LocalEvent) ReadArgs(args *g.EventArgs, evArgs *g.EventArgs) {
	fmt.Println("In ReadArgs function")

	// TODO: This method may not be neededs since we create a g.EventArgs in run.go - verify this

	evArgs.Profile = args.Profile
	evArgs.AppName = args.AppName
	evArgs.Target = args.Target
	evArgs.DryRun = args.DryRun
	evArgs.Keep = args.Keep
	evArgs.Vol = args.Vol
	evArgs.Src = args.Src
	evArgs.Rpt = args.Rpt
	evArgs.AppProfile = args.AppProfile
	evArgs.AppToolProf = args.AppToolProf
	evArgs.Loc = args.Loc
	evArgs.ParamsRaw = args.ParamsRaw
	// Note: evArgs.ToocConf is set in verifyRun as part of the runInfo struct
}

func (levent *LocalEvent) GetId() string {
	// Generate a type 4 UUID to use as an identifier of this run
	//u, err := uuid.NewV4()
	u := uuid.NewV4()
	//if err != nil {
	//	errorLog.Printf("Error generating run ID, errror was: %s", err)
	//	errorLog.Println("Unable to get a unique ID for this run, quitting")
	//	os.Exit(1)
	//}

	return u.String()
}

func (levent *LocalEvent) Startup(run *runInfo) {
	// Handle any defined startup tool runs for this named pipeline
	infoLog.Printf("In Startup stage of %v run...", run.name)

	// Determie if this run uses local filesystem or an ephemeral data volume
	// If there's no run.Vol, then create the ephemeral data volume
	if run.Vol == "none" {
		// Create the data volume
		dataVolume(run)
	}

	// Interate over the defined startup steps, running them in order
	for i := 0; i < len(run.startup); i++ {
		infoLog.Printf("Launching container for %v", run.startup[i].Tool)
		err := launchContainer(run.startup[i], run)
		if err != nil {
			warnLog.Printf("Error launching container during startup stage of run %s", run.name)
			errorLog.Printf("Error launching container was: %s", err)
			errorLog.Println("Unable to launch container for this run, quitting")
			os.Exit(1)
		}
	}

}

func (levent *LocalEvent) Pipeline(run *runInfo) {
	// Handle any defined pipeline tool runs for this named pipeline
	infoLog.Printf("In Pipeline stage of %v run...", run.name)
	fmt.Println("In Pipeline stage of ")

	// Interate over the defined pipeline steps, running them in order
	for i := 0; i < len(run.pipeline); i++ {
		infoLog.Printf("Launching container for %v", run.pipeline[i].Tool)
		err := launchContainer(run.pipeline[i], run)
		if err != nil {
			warnLog.Printf("Error launching container during pipeline stage of run %s", run.name)
			errorLog.Printf("Error launching container was: %s", err)
			errorLog.Println("Unable to launch container for this run, quitting")
			os.Exit(1)
		}
	}
}

func (levent *LocalEvent) Final(run *runInfo) {
	// Handle any defined final tool runs for this named pipeline
	infoLog.Printf("In Final stage of %v run...", run.name)
	fmt.Printf("In Final stage of %v\n run ", run.name)

	// Interate over the defined pipeline steps, running them in order
	for i := 0; i < len(run.final); i++ {
		infoLog.Printf("Launching container for %v", run.final[i].Tool)
		err := launchContainer(run.final[i], run)
		if err != nil {
			warnLog.Printf("Error launching container during final stage of run %s", run.name)
			errorLog.Printf("Error launching container was: %s", err)
			errorLog.Println("Unable to launch container for this run, quitting")
			os.Exit(1)
		}
	}
}

func (levent *LocalEvent) Cleanup(run *runInfo) {
	fmt.Println("In Cleanup function")
	// Not needed for local docker runs using --rm command-line option
}

// Vars and functions for gasp-docker
var logDir string = "./logs"
var traceLog *log.Logger
var infoLog *log.Logger
var warnLog *log.Logger
var errorLog *log.Logger

type runInfo struct {
	name         string
	startup      map[int]g.Tools
	pipeline     map[int]g.Tools
	final        map[int]g.Tools
	runevery     map[int]g.Tools
	toolProfiles map[string]g.SecTool
	sentParams   map[string]string
	runId        string
	detailed     *os.File // detailed logging
	dataVol      string   // The name of the ephemeral data volume used
	Vol          string   // The path of the local file system to use for /opt/appsecpipeline
	Src          string   // The path of the local file system to use for /opt/appsecpipeline/source
	Rpt          string   // The path of the local file system to use for /opt/appsecpipeline/reports
	runContainer []string // slice of containers run/launched in this run
	runVolume    []string // slice of volumes run/launced in this run
	keep         bool
	dryRun       bool
}

func listImages(ldock *LocalDockers) []Image {
	infoLog.Println("Getting list of Docker images available in repo")

	cmd := exec.Command("docker", "images")
	var sOut, sErr bytes.Buffer
	cmd.Stdout = &sOut
	cmd.Stderr = &sErr
	err := cmd.Run()
	if err != nil {
		errorLog.Printf("Error getting image list, errror was: %s", sOut.String())
		errorLog.Println("Unable to get a list of available Docker images, quitting")
		os.Exit(1)
	}
	//io.Copy(ldock.detailed, bytes.NewReader(sOut.Bytes()))
	// TODO: update all exec.Command calls to output stdout and stderr on errors
	//       also consider more info logs if needed after ^

	scanner := bufio.NewScanner(bytes.NewReader(sOut.Bytes()))
	images := make([]Image, 25)
	for scanner.Scan() {
		line := scanner.Text()
		// Strip off header line, process the rest
		if !strings.Contains(line, "REPOSITORY") {
			bits := strings.Fields(line)
			fn := bits[0] + ":" + bits[1]
			im := Image{
				fullName: fn,
				name:     bits[0],
				tag:      bits[1],
				id:       bits[2],
				created:  bits[3],
				size:     bits[4],
			}
			images = append(images, im)
		}
	}
	return images
}

func diffImages(r []Image, s *g.S) map[string]bool {
	infoLog.Println("Diff'ing needed against available Docker images")

	// Get a list of needed images with no duplicates, make all false aka not available
	inRepo := make(map[string]bool)
	for _, val := range s.T {
		inRepo[val.Docker] = false
	}

	// Compare needed against available in the repo
	for _, img := range r {
		// If the map has a key that's the name of an existing image, set to true aka available
		if _, ok := inRepo[img.fullName]; ok {
			inRepo[img.fullName] = true
			infoLog.Printf("Image %s is available on this system, no need to pull", img.fullName)
		}
	}
	return inRepo
}

func pullImages(i map[string]bool) {
	infoLog.Println("Pulling any needed images")

	// Run through missing list and pull images as needed
	c := 0
	for k, v := range i {

		if !v {
			// false aka need to pull this image
			fmt.Printf("Image needed, pulling image %s, this may take a bit.\n", k)
			infoLog.Printf("Image needed, pulling image %s\n", k)
			infoLog.Println("This will take a bit depending on network speeds")
			cmd := exec.Command("docker", "pull", k)
			var sOut, sErr bytes.Buffer
			cmd.Stdout = &sOut
			cmd.Stderr = &sErr
			err := cmd.Run()
			if err != nil {
				errorLog.Printf("Error pulling image %s, errror was: %s", k, sOut.String())
				errorLog.Println("Unable to pull required Docker image, quitting")
				os.Exit(1)
			}
			infoLog.Printf("Success pulling image %s\n", k)
			c += 1
		}
	}

	if c == 0 {
		infoLog.Println("No images to pull")
	}
	infoLog.Println("Completed pulling needed images")
}

func dataVolume(run *runInfo) {
	fmt.Println("In dataVolume")

	// Create a data volume which will hold source and results for this run
	fmt.Println("Calling launchVolume")
	vol, err := launchVolume(run)
	if err != nil {
		warnLog.Printf("Error creating data volme during startup stage of run %s", run.name)
		errorLog.Printf("Error creating data volme was: %s", err)
		errorLog.Println("Unable to data volume for this run, quitting")
		os.Exit(1)
	}
	fmt.Println("Created volume named ", vol)

	// Set results volume for this run
	run.dataVol = vol

	// Adjust the file permission of the volume to use the appsecpipeline user
	err = volumePerms(vol, run)
	if err != nil {
		warnLog.Printf("Error setting permissions on the data volme during startup stage of run %s", run.name)
		errorLog.Printf("Error setting volme permissions was: %s", err)
		errorLog.Println("Unable to set permissions on data volume for this run, quitting")
		os.Exit(1)
	}
}

func launchVolume(run *runInfo) (string, error) {
	// Create a data volume to use for tools results and possibly SAST targets
	vname := "data_" + run.runId

	fmt.Printf("run.dryRun is %v\n", run.dryRun)

	//	if !run.dryRun {
	//		fmt.Println("INSIDE IF")
	//		fmt.Printf("run.dryRun is %v\n", run.dryRun)
	//	} else {
	//		fmt.Println("INSIDE ELSE")
	//		fmt.Printf("run.dryRun is %v\n", run.dryRun)
	//	}

	if !run.dryRun {
		fmt.Println("NO DRY RUN - CREATING VOLUME")
		cmd := exec.Command("docker", "volume", "create", vname)
		var sOut, sErr bytes.Buffer
		cmd.Stdout = &sOut
		cmd.Stderr = &sErr
		err := cmd.Run()
		if err != nil {
			errorLog.Printf("Error creating data volume %s, errror was: %s", vname, sOut.String())
			errorLog.Println("Unable to create data volume, quitting")
			os.Exit(1)
		}
		io.Copy(run.detailed, bytes.NewReader(sOut.Bytes()))
	}
	infoLog.Printf("Success creating data volume %s\n", vname)

	return vname, nil
}

func volumePerms(vol string, run *runInfo) error {
	// Ajust the file permissions of the new volume so they are owned by the appsecpipeline user
	volMount := vol + ":/opt/appsecpipeline/"
	dName := "set-perms_" + run.runId

	// TODO: Add this as a tool + pipeline step
	// TODO: change this the the string slice format from launch container & infoLog the args - YAGA

	// Container can be any AppSec Pipeline image, since minimum named pipeline must have at least 1 tool for
	// the pipeline stage, we can safely set the container name to the first pipeline tool's container image
	//container := run.toolProfiles[(run.pipeline[0].Tool)].Docker
	container := "mtesauro/gasp-base:1.0.0" // TODO: Revert this
	if !run.dryRun {
		cmd := exec.Command("docker", "run", "-v", volMount, "--name", dName,
			"--user=root", "--rm", "--entrypoint", "chown", container,
			"-R", "appsecpipeline:appsecpipeline", "/opt/appsecpipeline")
		var sOut, sErr bytes.Buffer
		cmd.Stdout = &sOut
		cmd.Stderr = &sErr
		err := cmd.Run()
		if err != nil {
			errorLog.Printf("Error setting file permissions on data volume %s, errror was: %s\n%s", vol, sErr.String(), err)
			errorLog.Println("Unable to create data volume with needed file permissions, quitting")
			os.Exit(1)
		}
		io.Copy(run.detailed, bytes.NewReader(sOut.Bytes()))
	}
	infoLog.Printf("Successfully set file permissions on data volume %s\n", vol)

	return nil

}

func launchContainer(tool g.Tools, run *runInfo) error {
	// Run the provided tool from this portion of the named pipeline run
	dName := tool.Tool + "_" + run.runId

	// Deterine mounting for data volume(s) - local filesystem or emphemeral data volume
	volMount := ""
	if run.Vol == "none" {
		// Use the ephemeral data volume
		volMount = run.dataVol + ":/opt/appsecpipeline/"
	} else {
		// Use the provided local filesytem path
		volMount = run.Vol + ":/opt/appsecpipeline/"
	}

	// Build the command for this tool
	args := []string{
		"run",
		"-v", volMount,
	}

	// Keep or remove container based on -k/--keep flag
	if !run.keep {
		args = append(args, "--rm")
	}

	// If provided, mount the local filesystem path that has source code
	if run.Src != "none" {
		lv := run.Src + ":/opt/appsecpipeline/source"
		fmt.Printf("Local volume is:\n  =>%s<=\n", lv)
		args = append(args, "-v", lv)
	}

	//TODO: Do like the above for -r (reporting)
	if run.Rpt != "none" {
		rv := run.Rpt + ":/opt/appsecpipeline/source"
		fmt.Printf("Local volume is:\n  =>%s<=\n", rv)
		args = append(args, "-v", rv)
	}

	// Add more default args
	args = append(args, "--net=host", "--name", dName)

	//"--user", "root", Not needed with gasp dockers
	//"--entrypoint", Not needed with gasp dockers

	// Add the container for the current tool
	args = append(args, run.toolProfiles[tool.Tool].Docker)

	toolCmd := genToolCmd(tool.Tool, tool.ToolProfile, run)
	fmt.Printf("Tool Command is %+v\n", toolCmd)

	// Append the rest of the command args
	args = append(args, strings.Split(toolCmd, " ")...)

	// Log what was sent to docker for this run
	infoLog.Printf("ARGS sent to docker were %+v\n", args)
	fmt.Printf("ARGS sent to docker were %+v\n", args)

	if !run.dryRun {
		// Run the container
		cmd := exec.Command("docker", args...)
		var sOut, sErr bytes.Buffer
		cmd.Stdout = &sOut
		cmd.Stderr = &sErr
		err := cmd.Run()
		if err != nil {
			errorLog.Printf("Error launching container %s, errror was: %s\n%s\n%s", dName, sErr.String(), err, sOut.String())
			errorLog.Println("Unable to launch container for pipeline stage, quitting")
			os.Exit(1)
		}
		io.Copy(run.detailed, bytes.NewReader(sOut.Bytes()))
		// TODO: Write these to gasp-log
		fmt.Printf("StdOut is %v\n", sOut.String())
		fmt.Printf("StdErr is %v\n", sErr.String())
	}
	infoLog.Printf("Successfully launched container %s\n", dName)

	return nil
}

func genToolCmd(tool string, toolProf string, run *runInfo) string {
	cmd := ""

	// Pull out the commands for this tool - starting with the pre-command
	if pre, _ := run.toolProfiles[tool].Cmds["pre"]; len(pre) > 0 {
		// Add pre command since it exists
		cmd += cmdSub(pre, tool, run) + " "
	}
	// The exec or main command
	if exec, _ := run.toolProfiles[tool].Cmds["exec"]; len(exec) > 0 {
		cmd += cmdSub(exec, tool, run) + " "
	}
	// Check if report option is provided
	if rep, _ := run.toolProfiles[tool].Cmds["report"]; len(rep) > 0 {
		// Add report option
		cmd += cmdSub(rep, tool, run) + " "
	}
	// Substitue out any passed in values e.g. LOC=/opt/appsecpipeline/source
	prf := run.toolProfiles[tool].Pfls[toolProf]
	cmd += cmdSub(prf, tool, run) + " "
	// Check if a post is provided
	if post, _ := run.toolProfiles[tool].Cmds["post"]; len(post) > 0 {
		// Add post command
		cmd += " && " + cmdSub(post, tool, run) + " "
	}

	return cmd
}

func cmdSub(c string, tool string, run *runInfo) string {
	// Do substitutions to build a a new command
	newCmd := c

	// Check for { in the string sent for substitutions from secpipeline-config.yaml
	if strings.Contains(c, "{") {
		// Pull out the string inside of { }
		v := c[(strings.Index(c, "{") + 1):strings.Index(c, "}")]
		// Currently, only reportname and timestamp are supported in the yaml
		switch v {
		case "reportname":
			newCmd = strings.Replace(newCmd, "{reportname}", cmdSub(run.toolProfiles[tool].Cmds[v], tool, run), -1)
		case "timestamp":
			stamp := strconv.Itoa(int(time.Now().UnixNano()))
			// TODO: Check if Unix nanoseconds is right timestamp to use
			newCmd = strings.Replace(newCmd, "{timestamp}", stamp, -1)
		default:
			warnLog.Printf("Unknown {} substitution - %v", v)
			errorLog.Printf("No substituion made for %v", v)
		}
	}

	// Run through the command to send to the container and make any needed substitutions
	for _, word := range strings.Split(newCmd, " ") {
		if strings.Contains(word, "$") {
			// Subsitute out any variables
			var to []string
			cleanedParams := strings.Trim(run.sentParams[tool], " ")
			cleanedParams = strings.Replace(cleanedParams, "  ", " ", -1)
			for _, v := range strings.Split(cleanedParams, " ") {
				// Avoid any empty strings
				if len(v) > 0 {
					// Vars to swap out start with $
					cmdVar := word[strings.Index(word, "$")+1:]
					if strings.Contains(v, cmdVar) {
						// Match found, do the replacement
						to = strings.Split(string(v), "=")
						newCmd = strings.Replace(newCmd, word[strings.Index(word, "$"):], to[1], 1)
					}
				}
			}
		}
	}

	return newCmd
}

func verifyRun(ev *g.EventArgs, mstr *g.M, sec *g.S, run *runInfo) {
	// Sanity check the provided arguments vs the config files for any issues before starting the run
	fmt.Println("In verifyRun")

	// Move over needed command-line options
	run.keep = ev.Keep
	run.dryRun = ev.DryRun
	run.Vol = ev.Vol
	run.Src = ev.Src
	run.Rpt = ev.Rpt

	// Set the named pipeline for this run
	run.name = ev.Profile

	// Look for [app-name]-pipeline.yaml
	// TODO: Add a check for this file in the config directory in verifyRun() [app-name]-pipeline.yaml

	// Look for [app-name]-tool.yaml
	// TODO: Add a check for this file in the config directory in verifyRun() [app-name]-tool.yaml

	// Get a list of all tools used in this run and the parameters sent in the command-line
	tools := make([]string, 0)
	conf := make(map[string]string)
	tc := 0
	ts := make(map[int]g.Tools)
	// Collect startup tools and assign their options for this run
	for _, s := range mstr.Prof[ev.Profile].Startup {
		tools = append(tools, s.Tool)
		// Pull out the tool and options set in the startup profile for this run
		ts[tc] = s
		found := ""
		for sp, _ := range sec.T[s.Tool].Parameters {
			// For each parameter for this tool, add any that were provided in the command-line
			found += setOption(sp, ev.ParamsRaw)
			found += " "
		}
		conf[s.Tool] = (strings.TrimRight(found, " "))
		tc++
	}
	run.startup = ts

	// Collect pipeline tools and assign their options for this run
	tc = 0
	tp := make(map[int]g.Tools)
	for _, p := range mstr.Prof[ev.Profile].Pipeline {
		tools = append(tools, p.Tool)
		// Pull out the tool and options set in the pipeline profile for this run
		tp[tc] = p
		found := ""
		for pp, _ := range sec.T[p.Tool].Parameters {
			// For each parameter for this tool, add any that were provided in the command-line
			found += setOption(pp, ev.ParamsRaw)
			found += " "
		}
		conf[p.Tool] = (strings.TrimRight(found, " "))
		tc++
	}
	run.pipeline = tp

	// Collect final tools and assign their options for this run
	tc = 0
	tf := make(map[int]g.Tools)
	for _, f := range mstr.Prof[ev.Profile].Final {
		tools = append(tools, f.Tool)
		// Pull out the tool and options set in the pipeline profile for this run
		tf[tc] = f
		found := ""
		for fp, _ := range sec.T[f.Tool].Parameters {
			// For each parameter for this tool, add any that were provided in the command-line
			found += setOption(fp, ev.ParamsRaw)
			found += " "
		}
		conf[f.Tool] = (strings.TrimRight(found, " "))
		tc++
	}
	run.final = tf

	// Collect runevery tools and assign their options for this run
	tc = 0
	tr := make(map[int]g.Tools)
	for _, r := range mstr.Prof[ev.Profile].RunEvery {
		tools = append(tools, r.Tool)
		// Pull out the tool and options set in the pipeline profile for this run
		tr[tc] = r
		found := ""
		for rp, _ := range sec.T[r.Tool].Parameters {
			// For each parameter for this tool, add any that were provided in the command-line
			found += setOption(rp, ev.ParamsRaw)
			found += " "
		}
		conf[r.Tool] = (strings.TrimRight(found, " "))
		tc++
	}
	run.runevery = tr

	// Cycle through tools used in this run and pull their profiles from sec - the datastructure for secpipeline-config.yaml
	run.toolProfiles = make(map[string]g.SecTool)
	for _, tool := range tools {
		pullToolProfile(tool, run, sec)
	}

	// Verify that the option in the profile exists for the tool
	verifyOptions(run)

	// TODO: Verify that there's at least 1 tool defined in the pipeline stage
	//       - that's the smallest possible named pipeline

	// Set run's sentParams with the tool parameters provided by the command-line
	run.sentParams = conf

	//fmt.Printf("defectdojo's args are: %+v\n", run.sentParams["defectdojo"])
}

// If the needed parameter was sent in the command-line (arg) then
// return the matching NAME=value string, else return an empty string
func setOption(param string, arg string) string {

	// Split up the name=value pairs that were provided on the command-line
	vars := strings.Split(arg, " ")

	for _, v := range vars {
		// Try to match the command-line arg to a parameter needed by the current tool
		if strings.Contains(v, param) {
			// Tool parameter was sent as a commmand-line argument so return it
			return v
		}
	}

	// Tool parameter wasn't sent as a command-line arguement to return empty string
	return ""
}

// Take a tool name, get that profile from sec (secpipeline-config.yaml) and
// add it to current run struct (runInfo)
func pullToolProfile(tool string, run *runInfo, sec *g.S) {

	// Check the map for a key that's the current tool, set OK to true if it exists
	_, ok := sec.T[tool]
	if !ok {
		// Tool was in profile that doesn't have configuration, this is a fatal error
		warnLog.Printf("Tool '%s' was in the current profile but is not defined in secpipeline-config.yaml", tool)
		errorLog.Println("FATAL: Unable to find a tool profile for the tool requested in this pipeline run, quitting")
		os.Exit(1)
	}
	// Since the tool exists in sec (secpipeline-config.yaml), add its config to the current run
	run.toolProfiles[tool] = sec.T[tool]
}

// Verify that the option in the profile exists for the tool
// So if the profile has the 'git' tool and uses the option 'merge',
// secpipeline-config.yaml will be checked to ensure that option exists.  If not,
// it will fail fatally and log an error message.
func verifyOptions(run *runInfo) {
	infoLog.Println("In verifyOptions")

	// Check startup's tool profiles
	for _, s := range run.startup {
		infoLog.Printf("The '%v' named pipeline's startup stage included the '%v' tool with an option of '%v'\n", run.name, s.Tool, s.ToolProfile)
		infoLog.Printf("The option for '%v' in the master profile is %v\n", s.Tool, run.toolProfiles[s.Tool].Pfls[s.ToolProfile])
		_, ok := run.toolProfiles[s.Tool].Pfls[s.ToolProfile]
		if !ok {
			// Tool profile sent doesn't exist for tool
			warnLog.Printf("The '%v' option for '%v' was not found", s.ToolProfile, s.Tool)
			errorLog.Printf("FATAL: No option of '%v' is defined for '%v' in secpipeline-config.yaml, quitting", s.ToolProfile, s.Tool)
			os.Exit(1)
		}
	}

	// Check pipeline's tool profiles
	for _, p := range run.pipeline {
		infoLog.Printf("The '%v' named pipeline's pipeline stage included the '%v' tool with an option of '%v'\n", run.name, p.Tool, p.ToolProfile)
		infoLog.Printf("The option for '%v' in the master profile is %v\n", p.Tool, run.toolProfiles[p.Tool].Pfls[p.ToolProfile])
		_, ok := run.toolProfiles[p.Tool].Pfls[p.ToolProfile]
		if !ok {
			// Tool profile sent doesn't exist for tool
			warnLog.Printf("The '%v' option for '%v' was not found", p.ToolProfile, p.Tool)
			errorLog.Printf("FATAL: No option of '%v' is defined for '%v' in secpipeline-config.yaml, quitting", p.ToolProfile, p.Tool)
			os.Exit(1)
		}
	}

	// Check final's tool profiles
	for _, f := range run.final {
		infoLog.Printf("The '%v' named pipeline's final stage included the '%v' tool with an option of '%v'\n", run.name, f.Tool, f.ToolProfile)
		infoLog.Printf("The option for '%v' in the master profile is %v\n", f.Tool, run.toolProfiles[f.Tool].Pfls[f.ToolProfile])
		_, ok := run.toolProfiles[f.Tool].Pfls[f.ToolProfile]
		if !ok {
			// Tool profile sent doesn't exist for tool
			warnLog.Printf("The '%v' option for '%v' was not found", f.ToolProfile, f.Tool)
			errorLog.Printf("FATAL: No option of '%v' is defined for '%v' in secpipeline-config.yaml, quitting", f.ToolProfile, f.Tool)
			os.Exit(1)
		}
	}

	// Check runevery's tool profiles
	for _, r := range run.runevery {
		infoLog.Printf("The '%v' named pipeline's runevery stage included the '%v' tool with an option of '%v'\n", run.name, r.Tool, r.ToolProfile)
		infoLog.Printf("The option for '%v' in the master profile is %v\n", r.Tool, run.toolProfiles[r.Tool].Pfls[r.ToolProfile])
		_, ok := run.toolProfiles[r.Tool].Pfls[r.ToolProfile]
		if !ok {
			// Option sent doesn't exist for tool
			warnLog.Printf("The '%v' option for '%v' was not found", r.ToolProfile, r.Tool)
			errorLog.Printf("FATAL: No option of '%v' is defined for '%v' in secpipeline-config.yaml, quitting", r.ToolProfile, r.Tool)
			os.Exit(1)
		}
	}
}

func LoadPipeline(args *g.EventArgs) {
	// Start gasp-docker logging
	l := g.SetupLogging("gasp-docker", logDir, true)
	traceLog = l["trace"]
	infoLog = l["info"]
	warnLog = l["warn"]
	errorLog = l["error"]
	infoLog.Println("Logging setup for gasp-docker")

	// Set the directory location of the config files (master.yaml & secpipeline-config.yaml)
	subdir := "./spec"

	// Check Dependencies
	d := g.Deps{
		[]string{"docker"},
		[]string{"master.yaml", "secpipeline-config.yaml"},
		subdir,
		[]string{},
	}
	ld := g.LocalDeps{}
	ld.VerifyPrereqs(d)
	infoLog.Println("All dependencies needed for gasp-docker are available")

	// Read the configs to set things up
	lconf := g.LocalConfigs{"master.yaml", "secpipeline-config.yaml"}
	mstr := g.M{}
	lopts := g.ConfigOpts{"Local Files", subdir}
	lconf.ReadMaster(&mstr, lopts)
	sec := g.S{}
	lconf.ReadSecPipe(&sec, lopts)

	// Setup struct for tracking container images
	ldock := LocalDockers{}

	// Sync images so all needed tool images are in image repo
	ldock.SyncImages(&sec)
	//TODO: Look through yaml files to make sure they are consistent on image names & versions

	// handleEvent
	eArgs := g.EventArgs{}
	le := LocalEvent{}
	le.ReadArgs(args, &eArgs)

	// Verify the event's data against what's needed for this run
	// And set runInfo with this runs data if everything checks out
	//singleRun := new(runInfo)
	singleRun := runInfo{}
	verifyRun(&eArgs, &mstr, &sec, &singleRun)

	// Run the sent Named Profile
	singleRun.runId = le.GetId()

	// Initialize detailed logging
	fullPath := path.Join(logDir, (singleRun.runId + "_detailed.log"))
	dL, err := os.Create(fullPath)
	if err != nil {
		fmt.Printf("\nPlease create any directories needed to write logs to %v\n\n", logDir)
		log.Fatalf("Failed to open log file %s.  Error was:\n  %+v\n", fullPath, err)
	}
	singleRun.detailed = dL
	defer dL.Close()

	//	fmt.Printf("The value of keep is %v\n", singleRun.keep)
	//	fmt.Printf("The value of dryRun is %v\n", singleRun.dryRun)
	//	fmt.Printf("The type of keep is %T\n", singleRun.keep)
	//	fmt.Printf("The type of dryRun is %T\n", singleRun.dryRun)
	//os.Exit(0)

	// Run startup stage
	le.Startup(&singleRun)

	// Run pipeline stage
	le.Pipeline(&singleRun)

	// Run final stage
	le.Final(&singleRun)

	// Run cleanup stage - not needed for local dockers if the --rm options is used
	//le.Pipeline(&singleRun)

	// TODO: Add more meta to the detailed log - maybe push everything into the main log
	// TODO: Set a version number for that command line option

	// DEBUG INFO
	fmt.Println("\nDEBUG INFO\n")
	fmt.Println("Clean up dockers from ths run with:")
	//fmt.Printf("docker rm set-perms_%s\n", singleRun.runId)
	//fmt.Printf("docker rm git_%s\n", singleRun.runId)
	//fmt.Printf("docker rm cloc_%s\n", singleRun.runId)
	//fmt.Printf("docker rm bandit_%s\n", singleRun.runId)
	//fmt.Printf("docker rm defectdojo_%s\n", singleRun.runId)
	fmt.Printf("docker volume rm data_%s\n\n", singleRun.runId)
	os.Exit(0)

}
