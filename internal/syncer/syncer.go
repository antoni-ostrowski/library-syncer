package syncer

import (
	"os"
	"os/exec"
)

func SyncFiles() error {
	user := os.Getenv("RSYNC_USER")
	host := os.Getenv("RSYNC_HOSTNAME")
	dest := os.Getenv("RSYNC_DEST")
	src := os.Getenv("SONGS_PATH")
	cmd := exec.Command("rsync", "-avz", "--delete", src, user+"@"+host+":"+dest)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
