# geteduroam Linux client

This repository contains the source code for the geteduroam Linux client. Currently WIP.

[![Get it on Flathub](https://flathub.org/api/badge?locale=en)](https://flathub.org/apps/app.eduroam.geteduroam)

Note that currently it only works with NetworkManager. But support for e.g. wpa-supplicant and iwd is planned.

This is a fork of [geteduroam/linux-app](https://github.com/geteduroam/linux-app), see [changes in this fork](#changes-in-this-fork).

# Install on Arch Linux
For Arch Linux and distributions based on it (CachyOS, EndeavourOS, Manjaro, ...) there is an install script. It builds a
pacman package from the source, with the GUI, the CLI and the notification checker:
```bash
git clone https://github.com/WinTone01/linux-app.git
cd linux-app
./install-arch.sh
```

The script installs the build dependencies and asks for your password when it is needed, do not run it as root. A
version that was previously put in `~/.local` by hand is removed, your connection profiles are kept.

To update, run `git pull` and then the script again. To uninstall:
```bash
./install-arch.sh --uninstall
```

# Install through DEB/RPM
Note that these are the official packages, they do not have the changes in this fork.

To install the client using official packages, go to the [GitHub
releases page](https://github.com/geteduroam/linux-app/releases) and
pick DEB or RPM. These files can be saved and double clicked in your
file manager to install.

# Manual install
This section, explains the steps needed to manually build the client. We go over the CLI client and GUI client.
We also have a small binary that is used for sending of notifications, which we will explain too.

## Dependencies
- Go >= 1.18
- Make
- GTK >= 4.06 (for the GUI)
- Libadwaita >= 1.1 (for the GUI)
- libnotify (for notifications)
- NetworkManager

## CLI
To build the CLI client run:
```bash
make build-cli
```

This outputs the CLI to `./geteduroam-cli`, move this to somewhere in your `$PATH`, e.g. `/usr/bin`.

During development, the CLI can be build and run with the command:
```bash
make run-cli
```

## GUI
To build the GUI client run:
```bash
make build-gui 
```

This outputs the GUI to `./geteduroam-gui`, move this to somewhere in your `$PATH`, e.g. `/usr/bin`.

During development, the GUI can be build and run with the command:
```bash
make run-gui
```

## Notifications
For eduroam profiles that use TLS client certificates, the client can
warn for imminent expiry. As the geteduroam client is not always open,
we provide Systemd user files that check daily for imminent
expiry. These systemd user files run the `./cmd/geteduroam-notifcheck/` binary.

To build this binary, run:
```bash
make build-notifcheck
```

This outputs this binary to `./geteduroam-notifcheck`, move this somewhere in your `$PATH`, e.g. `/usr/bin/`.

During development, the notifcheck binary build and run with the command:
```bash
make run-notifcheck
```

To then set this up with Systemd, make sure that the
`systemd/user` service and timer files are in a location that the
systemd user daemon can find it. E.g. move them to `/etc/systemd/user`
or `~/.config/systemd/user`. The DEB and RPM packages do this
automatically. Note that when moving these files, make sure to reload systemd with `systemctl --user daemon-reload`.
Also note that these files hard-code the path to your `geteduroam-notifcheck` binary, in the service files we assume it is in `/usr/bin/`.

Contributions are welcome to support other daemons.

# Unwall
If [Unwall](https://github.com/WinTone01/Unwall) is installed, the client asks after adding a profile if it should connect
and set up Unwall for the network. If you agree:
- Encrypted DNS is turned on with the first provider that can be reached (Quad9, Cloudflare or Google). It is turned off
  again if names do not resolve with it
- The Unwall engine is started, if it is not running
- The current strategy is saved for the network, if there is none saved yet

This asks for your password using polkit. Nothing happens if Unwall is not installed or if you answer no. To do this
later for a profile that was already added:
```bash
geteduroam-cli --unwall
```
Check if the rules of the network allow this, your connection is tied to your account.

# Changes in this fork
- Access points in WPA1/WPA2 mixed mode work: these use TKIP for broadcast traffic, which NetworkManager refused. Your own
  traffic still has to use the cipher that the organization wants (CCMP)
- RADIUS servers that only have their name in the common name of the certificate work. The certificate and the name are
  still verified
- The username is checked against the realm of the organization, e.g. `user` is refused if it has to be
  `user@example.org` or `user@sub.example.org`
- The client works on slow networks: longer timeouts, the list of organizations is cached on disk and the GUI does not
  freeze while it is loading
- The GUI only uses GTK from the UI thread, and no longer unrefs the validity label twice
- Fixed crashes with a TLS profile without a client certificate, an empty state file and an invalid organization logo
- The CLI exits when there is no more input instead of looping forever

# License
[BSD 3](./LICENSE)
