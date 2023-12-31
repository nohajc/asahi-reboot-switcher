# asahi-reboot-switcher
GUI for asahi-bless - reboot to macOS system tray menu

## Install dependencies

```
sudo dnf install gcc make pkg-config gtk3-devel asahi-bless
```

### If asahi-bless version <= 0.2.1

```
git clone https://github.com/WhatAmISupposedToPutHere/asahi-nvram.git
cd asahi-nvram/asahi-bless
cargo install --path .
sudo cp ~/.cargo/bin/asahi-bless /usr/local/bin/
```

Also make sure `/usr/local/bin` is in `PATH` before `/usr/bin` or uninstall the asahi-bless package first.

### Gnome

The following extension must also be installed and enabled:
https://extensions.gnome.org/extension/615/appindicator-support/

## Build & install asahi-reboot-switcher
```
make
sudo make install
```

## How to use

Launch `Restart in macOS (tray icon)` from application menu. Next time it will start automatically after system login.

## Notes

Tested in KDE and Gnome.

### Gnome

If the tray icon is rendered incorrectly (with a green tint), go to Extension Manager, open extension preferences of `AppIndicator and KStatusNotifierItem Support` and set brightness to max (1.0).
