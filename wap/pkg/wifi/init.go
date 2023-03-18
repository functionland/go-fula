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
	"github.com/functionland/go-fula/wap/pkg/config"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("fula/wap/wifi")

const maxRetries = 4

var TimeLimit = 10 * time.Second

var conclusionMessages = map[string]string{}
var j *sdjournal.Journal
var workingDirectory string

var connectCommandsLinux = []string{
	`sudo killall wpa_supplicant`,
	fmt.Sprintf("sudo wpa_supplicant -B -i %s -c /etc/wpa_supplicant/wpa_supplicant.conf", config.IFFACE_CLIENT),
}

var connectRetryCommandsLinux = []string{
	fmt.Sprintf("sudo wpa_cli -i %s RECONFIGURE", config.IFFACE_CLIENT),
	fmt.Sprintf("sudo ifconfig %s up", config.IFFACE_CLIENT),
}
var disableCommands = []string{
	"sudo systemctl stop dnsmasq",
	"sudo systemctl stop hostapd",
	"sudo systemctl disable hostapd",
	fmt.Sprintf("sudo iw dev %s del", config.IFFACE),
	"sudo systemctl restart dhcpd",
}
var enableCommands = []string{
	fmt.Sprintf("sudo iw dev %s interface add %s type __ap", config.IFFACE_CLIENT, config.IFFACE),
	"sudo systemctl start dhcpcd",
	"sudo systemctl enable hostapd",
	"sudo systemctl unmask hostapd",
	"sudo systemctl start hostapd",
	"sudo systemctl start dnsmasq",
}

func init() {
	// Working Directory
	var err error
	workingDirectory, err = os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	/* 	r, err := sdjournal.NewJournalReader(sdjournal.JournalReaderConfig{
	   		Since: time.Duration(0) * time.Second,
	   	})
	   	if err != nil {
	   		log.Fatalf("Error opening journal: %s", err)
	   	}

	   	if r == nil {
	   		log.Fatal("Got a nil journalctl reader")
	   	} */
	j, err = sdjournal.NewJournal()
	if err != nil {
		log.Fatalf("Error opening journal: %s", err)
	}

}
func runJournalCtlCommands(ctx context.Context, commands []string) error {
	var err error
	for i := 0; i <= maxRetries; i++ {
		closers := []context.CancelFunc{}
		var errJournal error
		for _, cmd := range disableCommands {
			ctxJournal, close := context.WithCancel(ctx)
			closers = append(closers, close)
			errJournal = execWithJournalctlCallback(ctxJournal, cmd, nil)
			if errJournal != nil {
				log.Error(err)
				break
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("context error: %v", ctx.Err())
			default:
			}
		}

		if errJournal == nil {
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

func runCommand(ctx context.Context, commands string) (stdout, stderr string, err error) {
	log.Errorw("running", "commands", commands)
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
	stdout, stderr, err := runCommand(ctx, command)
	log.Infof("stdout: %s, stderr: %s", stdout, stderr)
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
