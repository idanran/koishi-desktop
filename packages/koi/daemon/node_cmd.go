package daemon

import (
	"bufio"
	log "github.com/sirupsen/logrus"
	"koi/config"
	"koi/util"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	// Log
	lKoishi = log.WithField("package", "koishi")
)

type NodeCmdOut struct {
	IsErr bool
	Text  string
}

type NodeCmd struct {
	Cmd *exec.Cmd
	Out *chan NodeCmdOut
}

func RunNode(
	entry string,
	args []string,
	dir string,
) error {
	args = append([]string{entry}, args...)
	return RunNodeCmd("node", args, dir)
}

func ResolveYarn() (string, error) {
	yarnPath, err := util.Resolve(config.Config.InternalNodeExeDir, "yarn.cjs")
	if err != nil {
		l.Error("Cannot resolve yarn.")
		return "", err
	}
	return yarnPath, nil
}

func RunYarn(
	args []string,
	dir string,
) error {
	yarnPath, err := ResolveYarn()
	if err != nil {
		return err
	}
	return RunNode(yarnPath, args, dir)
}

func RunNodeCmd(
	nodeExe string,
	args []string,
	dir string,
) error {
	cmd, err := CreateNodeCmd(nodeExe, args, dir)
	if err != nil {
		return err
	}
	return cmd.Run()
}

func CreateNodeCmd(
	nodeExe string,
	args []string,
	dir string,
) (*NodeCmd, error) {
	l.Debug("Getting env.")
	env := os.Environ()

	if config.Config.UseDataHome {
		l.Debug("Now replace HOME/USERPROFILE.")
		for {
			notFound := true
			for i, e := range env {
				if strings.HasPrefix(e, "HOME=") || strings.HasPrefix(e, "USERPROFILE=") {
					env = append(env[:i], env[i+1:]...)
					notFound = false
					break
				}
			}

			if notFound {
				break
			}
		}

		env = append(env, "HOME="+config.Config.InternalHomeDir)
		env = append(env, "USERPROFILE="+config.Config.InternalHomeDir)
		l.Debugf("HOME=%s", config.Config.InternalHomeDir)

		if runtime.GOOS == "windows" {
			l.Debug("Now replace APPDATA.")
			for {
				notFound := true
				for i, e := range env {
					if strings.HasPrefix(e, "APPDATA=") {
						env = append(env[:i], env[i+1:]...)
						notFound = false
						break
					}
				}

				if notFound {
					break
				}
			}

			roamingPath := filepath.Join(config.Config.InternalHomeDir, "AppData", "Roaming")
			env = append(env, "APPDATA="+roamingPath)
			l.Debugf("APPDATA=%s", roamingPath)

			l.Debug("Now replace LOCALAPPDATA.")
			for {
				notFound := true
				for i, e := range env {
					if strings.HasPrefix(e, "LOCALAPPDATA=") {
						env = append(env[:i], env[i+1:]...)
						notFound = false
						break
					}
				}

				if notFound {
					break
				}
			}

			localPath := filepath.Join(config.Config.InternalHomeDir, "AppData", "Local")
			env = append(env, "LOCALAPPDATA="+localPath)
			l.Debugf("LOCALAPPDATA=%s", localPath)
		}
	}

	if config.Config.UseDataTemp {
		l.Debug("Now replace TMPDIR/TEMP/TMP.")
		for {
			notFound := true
			for i, e := range env {
				if strings.HasPrefix(e, "TMPDIR=") || strings.HasPrefix(e, "TEMP=") || strings.HasPrefix(e, "TMP=") {
					env = append(env[:i], env[i+1:]...)
					notFound = false
					break
				}
			}

			if notFound {
				break
			}
		}

		env = append(env, "TMPDIR="+config.Config.InternalTempDir)
		env = append(env, "TEMP="+config.Config.InternalTempDir)
		env = append(env, "TMP="+config.Config.InternalTempDir)
		l.Debugf("TEMP=%s", config.Config.InternalTempDir)
	}

	l.Debug("Now replace PATH.")
	pathEnv := ""
	for {
		notFound := true
		for i, e := range env {
			if strings.HasPrefix(e, "PATH=") {
				pathEnv = e[5:]
				env = append(env[:i], env[i+1:]...)
				notFound = false
				break
			}
		}

		if notFound {
			break
		}
	}
	var pathSepr string
	if runtime.GOOS == "windows" {
		pathSepr = ";"
	} else {
		pathSepr = ":"
	}
	if pathEnv != "" && !config.Config.Strict {
		pathEnv = config.Config.InternalNodeExeDir + pathSepr + pathEnv
	} else {
		pathEnv = config.Config.InternalNodeExeDir
	}
	env = append(env, "PATH="+pathEnv)
	l.Debugf("PATH=%s", pathEnv)

	koiEnv := "KOI=" + config.Version
	env = append(env, koiEnv)
	l.Debug(koiEnv)

	l.Debugf("PWD=%s", dir)

	l.Debug("Now constructing NodeCmd.")
	cmdPath := filepath.Join(config.Config.InternalNodeExeDir, nodeExe)
	cmdArgs := []string{cmdPath}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Cmd{
		Path: cmdPath,
		Args: cmdArgs,
		Env:  env,
		Dir:  dir,
	}

	l.Debug("Now constructing io.")
	ch := make(chan NodeCmdOut)
	// No need to close chan.
	// https://stackoverflow.com/questions/8593645/is-it-ok-to-leave-a-channel-open

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		l.Error("Err constructing cmd.StdoutPipe():")
		l.Error(err)
		return nil, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		l.Error("Err constructing cmd.StderrPipe():")
		l.Error(err)
		return nil, err
	}
	stdoutScanner := bufio.NewScanner(stdoutPipe)
	stderrScanner := bufio.NewScanner(stderrPipe)
	go func() {
		for stdoutScanner.Scan() {
			s := stdoutScanner.Text() + util.ResetCtrlStr
			lKoishi.Info(s)
			// Non-blocking sender with empty statement.
			// https://dev.to/calj/channel-push-non-blocking-in-golang-1p8g
			select {
			case ch <- NodeCmdOut{
				IsErr: false,
				Text:  s,
			}:
			default:
			}
		}
		if err := stdoutScanner.Err(); err != nil {
			l.Error("Err reading stdout:")
			l.Error(err)
		}
	}()
	go func() {
		for stderrScanner.Scan() {
			s := stderrScanner.Text() + util.ResetCtrlStr
			lKoishi.Info(s)
			select {
			case ch <- NodeCmdOut{
				IsErr: false,
				Text:  s,
			}:
			default:
			}
		}
		if err := stderrScanner.Err(); err != nil {
			l.Error("Err reading stdout:")
			l.Error(err)
		}
	}()

	return &NodeCmd{
		Cmd: &cmd,
		Out: &ch,
	}, nil
}

func (c *NodeCmd) Run() error {
	l.Debug("Now run NodeCmd.")
	// Can use c.Cmd.Run() instead,
	// but remain NodeCmd method call for future refactoring.
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

func (c *NodeCmd) Start() error {
	l.Debug("Now start NodeCmd.")
	return c.Cmd.Start()
}

func (c *NodeCmd) Wait() error {
	l.Debug("Now wait NodeCmd.")
	return c.Cmd.Wait()
}
