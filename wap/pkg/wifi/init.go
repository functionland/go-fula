package wifi

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/sdjournal"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("fula/wap/wifi")

const maxRetries = 4

var TimeLimit = 10 * time.Second

var conclusionMessages = map[string]string{}
var j *sdjournal.Journal
var workingDirectory string

func init() {
	// Working Directory
	var err error
	workingDirectory, err = os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	// Check if NetworkManager is installed
	_, err = exec.LookPath("nmcli")
	if err != nil {
		log.Fatal("nmcli (NetworkManager) not found")
	}
	// Check if iw is installed
	// _, err := exec.LookPath("iw")
	// if err != nil {
	// 	log.Fatal("iw not found")
	// }

}

func runCommands(ctx context.Context, commands []string) error {
	for i := 0; i <= maxRetries; i++ {
		closers := []context.CancelFunc{}
		var err error
		for _, cmd := range commands {
			ctxJournal, close := context.WithCancel(ctx)
			closers = append(closers, close)
			_, _, err = runCommand(ctxJournal, cmd)
			if err != nil {
				log.Errorf("executing multiple commands all at once: %v", err)
				break
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("context error: %v", ctx.Err())
			default:
			}
		}

		if err == nil {
			break
		}

		select {
		case <-ctx.Done():
			for _, closer := range closers {
				if closer != nil {
					closer()
				}
			}
			return fmt.Errorf("context error: %v", ctx.Err())
		default:
		}
	}
	return nil
}

// exported
func RunCommand(ctx context.Context, commands string) (stdout, stderr string, err error) {
	return runCommand(ctx, commands)
}

func runCommand(ctx context.Context, commands string) (stdout, stderr string, err error) {
	log.Infow("running", "commands", commands)
	command := strings.Fields(commands)
	cmd := exec.Command(command[0])
	if len(command) > 0 {
		cmd = exec.Command(command[0], command[1:]...)
	}
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err = cmd.Start()
	if err != nil {
		return
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case <-ctx.Done():
		err = cmd.Process.Kill()
	case err = <-done:
		stdout = outb.String()
		stderr = errb.String()
	}
	return
}

func execWithJournalctlCallback(ctx context.Context, command string, conclusionMessage *string) error {
	err := j.SeekTail()
	if err != nil {
		return fmt.Errorf("seeking to the end of the journal: %s", err)
	}
	_, _, err = runCommand(ctx, command)
	if err != nil {
		return fmt.Errorf("running command(%s): %s", command, err)
	}
	for {
		entryExist, _ := j.Next()
		for entryExist > 0 {
			msg, err := j.GetData(sdjournal.SD_JOURNAL_FIELD_MESSAGE)
			if err != nil {
				log.Errorf("reading the message field: %s", err)
				break
			}
			cmdLine, err := j.GetData(sdjournal.SD_JOURNAL_FIELD_CMDLINE)
			if err != nil {
				log.Errorf("reading the cmdline field:: %s", err)
				break

			}
			if strings.Contains(msg, "pam_unix(sudo:session): session closed for user root") && cmdLine == command {
				return nil
			}
			entryExist, _ = j.Next()
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("context error: %v", ctx.Err())
		case <-time.After(3 * time.Second):
		}
	}

}
