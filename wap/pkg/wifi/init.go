package wifi

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("fula/wap/wifi")

const maxRetries = 4

var TimeLimit = 25 * time.Second

func init() {
	// Working Directory
	//var err error

	// Check if NetworkManager is installed
	//_, err = exec.LookPath("nmcli")
	//if err != nil {
	//log.Fatal("nmcli (NetworkManager) not found")
	//}
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
	cmd := exec.CommandContext(ctx, command[0])
	if len(command) > 0 {
		cmd = exec.CommandContext(ctx, command[0], command[1:]...)
	}
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err = cmd.Start()
	if err != nil {
		return stdout, stderr, err
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case <-ctx.Done():
		if killErr := cmd.Process.Kill(); killErr != nil {
			log.Errorw("Failed to kill process", "error", killErr)
			stderr = errb.String()
			return outb.String(), stderr, killErr
		}
	case err = <-done:
		stdout = outb.String()
		stderr = errb.String()
	}
	return stdout, stderr, err
}

func runCommandDirect(cmd *exec.Cmd) error {
	// Output the command being executed for debugging purposes
	// fmt.Println("Executing:", cmd.String())

	// Start the command and wait for it to finish
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}
