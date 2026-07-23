package syncer

import (
	"os"
	"os/exec"
)

func SyncFiles() error {
	key := os.Getenv("SSH_KEY")
	user := os.Getenv("RSYNC_USER")
	host := os.Getenv("RSYNC_HOSTNAME")
	dest := os.Getenv("RSYNC_DEST")
	src := os.Getenv("RSYNC_SRC")
	cmd := exec.Command("rsync", "-avz", "--delete", "-e", "ssh -i "+key, src, user+"@"+host+":"+dest)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
