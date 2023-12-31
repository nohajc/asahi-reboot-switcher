package main

import (
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/nohajc/asahi-reboot-switcher/asahibless"
	"github.com/nohajc/asahi-reboot-switcher/dialog"
	"github.com/nohajc/asahi-reboot-switcher/util"
	"github.com/nohajc/systray"
)

func setupAutostart(homeDir string) {
	autostartDir := filepath.Join(homeDir, ".config", "autostart")
	autostartFile := filepath.Join(autostartDir, "asahi-reboot-switcher.desktop")

	// Check if the autostart file already exists
	if _, err := os.Stat(autostartFile); os.IsNotExist(err) {
		// Create the autostart directory if it doesn't exist
		confirmed := dialog.
			Question("Restart in macOS tray icon will be set to autostart on login.").
			Title("Confirm autostart").
			Run()
		if !confirmed {
			return
		}
		if err := os.MkdirAll(autostartDir, 0755); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to create autostart directory:", err)
			return
		}

		// Open the source file for reading
		srcFile, err := os.Open("/usr/share/applications/asahi-reboot-switcher.desktop")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to open source file:", err)
			return
		}
		defer srcFile.Close()

		// Create the destination file for writing
		dstFile, err := os.Create(autostartFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to create destination file:", err)
			return
		}
		defer dstFile.Close()

		// Copy the contents from the source file to the destination file
		if _, err := io.Copy(dstFile, srcFile); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to copy file contents:", err)
			return
		}

		fmt.Println("Autostart file copied successfully.")
	}
}

func main() {
	currUser, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	_ = util.RequireCommand("pkexec")

	if len(os.Args) > 1 {
		callAsahiBless(os.Args[1:])
		return
	}

	if currUser.Uid == "0" {
		fmt.Fprintln(os.Stderr, "Should not run as root, exiting...")
		os.Exit(1)
	}

	setupAutostart(currUser.HomeDir)

	sctx := &SystrayContext{}
	err = sctx.loadVolumes()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	systray.Run(sctx.onReady, func() {})
}

func requestReboot() error {
	if os.Getenv("XDG_CURRENT_DESKTOP") == "KDE" {
		cmd := exec.Command("qdbus", "org.kde.ksmserver", "/KSMServer", "logout", "1", "1", "3")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	if os.Getenv("XDG_CURRENT_DESKTOP") == "GNOME" {
		cmd := exec.Command("gnome-session-quit", "--reboot")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// fallback
	cmd := exec.Command("pkexec", "reboot")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

var allowedBlessPaths = []string{"/usr/local/bin", "/usr/bin"}
var asahiBlessCmd = util.RequireCommand("asahi-bless", allowedBlessPaths...)

func callAsahiBless(args []string) {
	{
		cmd := exec.Command(asahiBlessCmd, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to set boot volume:", err)
		}
	}
}

//go:embed asahi-reboot-switcher.png
var appIcon []byte

type SystrayContext struct {
	volumes          []asahibless.Volume
	volMenuItemGroup *systray.MenuItemRadioGroup
	activeVolIdx     int
}

func (sc *SystrayContext) loadVolumes() error {
	volumes, err := asahibless.ListVolumes()
	sc.volumes = volumes
	if sc.volMenuItemGroup == nil {
		return nil
	}
	// fmt.Printf("Volumes: %#v\n", volumes)
	for _, v := range volumes {
		i := v.Idx - 1
		if v.Active {
			sc.volMenuItemGroup.Check(i)
			break
		}
	}
	return err
}

func (sc *SystrayContext) onReady() {
	systray.SetTemplateIcon(appIcon, appIcon)
	// systray.SetTitle("Asahi Reboot Switcher")
	systray.SetTooltip("Restart in macOS (tray icon)")

	mReboot := systray.AddMenuItem("Restart in macOS...", "")
	systray.AddSeparator()
	mLabel := systray.AddMenuItem("Default startup disk:", "")
	mLabel.Disable()

	volMenuItemGroup := systray.AddMenuItemRadioGroup()
	activeIdx := 0
	for i, v := range sc.volumes {
		if v.Active {
			activeIdx = i
		}
		_ = volMenuItemGroup.AddItem(v.ShortName(), "")
	}
	volMenuItemGroup.Check(activeIdx)

	go func() {
		for volIdx := range volMenuItemGroup.ClickedCh {
			if volIdx != sc.activeVolIdx {
				confirmed := dialog.
					Question("Change default startup disk to %s?", sc.volumes[volIdx].ShortName()).
					Title("Confirm startup disk change").
					OKButton("Change").
					Run()
				if confirmed {
					asahibless.SetBoot(volIdx + 1)
					sc.activeVolIdx = volIdx
				}
			}
			sc.loadVolumes()
		}
	}()
	sc.volMenuItemGroup = volMenuItemGroup
	sc.activeVolIdx = activeIdx

	systray.AddSeparator()
	mQuitOrig := systray.AddMenuItem("Quit", "Quit application")

	for {
		select {
		case <-mReboot.ClickedCh:
			sc.rebootToMacOS()
		case <-mQuitOrig.ClickedCh:
			confirmed := dialog.
				Question("Quit Restart in macOS tray icon?").
				Title("Confirm quitting").
				OKButton("Quit").
				Run()
			if confirmed {
				fmt.Println("Quit")
				systray.Quit()
			}
		}
	}
}

func (sc *SystrayContext) isMacOSActive() bool {
	for _, v := range sc.volumes {
		if v.Active && strings.Contains(v.ShortName(), "Macintosh") {
			return true
		}
	}
	return false
}

func (sc *SystrayContext) rebootToMacOS() {
	if !sc.isMacOSActive() { // TODO: also check if override not already set - isMacOSNext()
		fmt.Println("macOS is not active, setting next boot override...")
		err := asahibless.SetBootMacOS(true)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
	} else {
		fmt.Println("macOS is already active, rebooting...")
	}

	// time.Sleep(1 * time.Second)

	{
		err := requestReboot()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to reboot to macOS:", err)
		}
	}
}
