# Kagami: A Comprehensive System for Automated Debian/Ubuntu ISO Synthesis

## Abstract

Kagami is a specialised orchestration framework designed for the deterministic synthesis of Debian and Ubuntu-based operating system images. The system prioritises the deployment of vanilla desktop environments and the implementation of permanent restrictions on the snapd package management daemon where applicable. Through a modular, configuration-driven architecture, Kagami enables the generation of streamlined, high-performance distributions tailored to specific computational requirements.

## Etymology and Nomenclature

The designation "Kagami" derives from the Japanese term for "mirror" (鏡). This nomenclature serves a dual purpose: the system is designed to provide an undistorted reflection of the user's specified configuration within the resulting installation medium, and the project acknowledges an inspiration from the character Kagami Hiiragi of the anime series "Lucky Star."

## Functional Domain and Features

- Vanilla desktop environment synthesis supporting nine distinct environments (GNOME, KDE Plasma, Xfce, and others)
- Permanent snapd constraint via a seven-layer defensive architecture (Ubuntu builds)
- Automated dependency resolution and installation
- Cross-architecture support for both UEFI and BIOS boot protocols (x86_64)
- Full Debian live system support using `live-boot` and `live-config`
- Interactive configuration wizard for guided ISO specification

## Debian vs Ubuntu Architecture

| Aspect | Ubuntu | Debian |
|---|---|---|
| Live system | casper | live-boot + live-config |
| Live directory | /casper/ | /live/ |
| Boot parameter | boot=casper | boot=live |
| EFI package | grub-efi-amd64-signed | grub-efi-amd64 |
| Installer | ubiquity or calamares | calamares |
| Desktop meta | vanilla packages | task-*-desktop |
| Snapd blocking | 7-layer enforcement | not applicable |

## System Prerequisites

### Distribution Support

Kagami is optimised for execution on APT-based host environments: Ubuntu (LTS and current interim releases), Debian (Stable, Testing, and Unstable branches), and derivative distributions (Linux Mint, Elementary OS, etc.).

Non-APT distributions (Fedora, Arch, openSUSE) and APT-RPM based systems (ALT Linux) are not supported as build hosts.

For non-APT users, employ Distrobox (via Docker or Podman) to create a compatible Ubuntu or Debian container. Ensure the container maps your home directory to preserve workspace access.

### Computational Requirements

- Memory: minimum 4 GB allocated RAM; 8 GB recommended for concurrent build processes
- Storage: minimum 15 GB of available disk space for the chroot environment and ISO synthesis
- Privileges: root access is mandatory for filesystem manipulation and package management

## Installation

### Source-Based

```
git clone https://github.com/jimed-rand/kagami.git
cd kagami
make build
sudo make install
```

### Dependency Installation

```
make check-prereqs
sudo make install-deps
```

Required utilities: `debootstrap`, `squashfs-tools`, `xorriso`, `grub-pc-bin`, `grub-efi-amd64-bin`, `mtools`, `dosfstools`, `isolinux`, `syslinux`.

## Operational Usage

### 1. Build the Binary

```
make build
```

### 2. Install Build Dependencies

```
sudo ./kagami --install-deps
```

### 3. Configuration Wizard

```
sudo ./kagami --wizard
```

The wizard generates a `.json` configuration file and optionally initiates the build immediately.

### 4. Direct Build Execution

```
sudo ./kagami --config path/to/config.json
```

## CLI Reference

```
--config       Path to the JSON configuration file
--wizard       Launch the interactive configuration wizard
--install-deps Install all required build dependencies
--check-deps   Verify system build dependencies
--release      Override target release codename
--output       Define the output ISO file path
--workdir      Specify the build workspace directory
--mirror       Override the APT repository mirror URL
--block-snapd  Apply permanent snapd suppression (default: true)
--interactive  Enable interactive package selection during build
--version      Display version and runtime information
```

## Configuration Schema

The JSON configuration file supports both Ubuntu and Debian targets. The `distro` field is required; if absent, it is inferred from the `release` codename and `mirror` URL.

```json
{
  "distro": "debian",
  "release": "bookworm",
  "system": {
    "hostname": "my-debian",
    "block_snapd": false,
    "architecture": "amd64",
    "locale": "en_US.UTF-8",
    "timezone": "UTC"
  },
  "repository": {
    "mirror": "http://deb.debian.org/debian/",
    "use_proposed": false
  },
  "packages": {
    "essential": ["sudo", "live-boot", "live-boot-initramfs-tools", "live-config", "live-config-systemd", "..."],
    "additional": ["vim", "curl", "wget"],
    "desktop": "xfce",
    "kernel": "linux-image-amd64",
    "remove_list": [],
    "enable_flatpak": false
  },
  "installer": {
    "type": "calamares"
  },
  "network": {
    "manager": "network-manager"
  },
  "security": {
    "enable_firewall": false,
    "block_snapd_forever": false,
    "disable_services": []
  }
}
```

## Debian Essential Package Manifest

The following packages constitute the mandatory live system foundation for Debian builds:

```
sudo  live-boot  live-boot-initramfs-tools  live-config  live-config-systemd
discover  laptop-detect  os-prober  network-manager  net-tools
wireless-tools  wpasupplicant  locales  grub-common  grub-pc
grub-pc-bin  grub2-common  grub-efi-amd64  shim-signed  mtools  binutils
```

Note that `casper`, `grub-gfxpayload-lists`, and `ubuntu-standard` are Ubuntu-specific and must not be used in Debian configurations.

## Desktop Environments

### Ubuntu (vanilla package selection)

| Desktop | Display Manager | Idle Memory | Approximate Storage |
|---|---|---|---|
| GNOME | GDM3 | ~800 MB | ~3.0 GB |
| KDE Plasma | SDDM | ~600 MB | ~2.5 GB |
| Xfce | LightDM | ~400 MB | ~1.5 GB |
| LXQt | SDDM | ~300 MB | ~1.0 GB |
| MATE | LightDM | ~400 MB | ~1.5 GB |

### Debian (task metapackages)

| Desktop | Package |
|---|---|
| GNOME | task-gnome-desktop |
| KDE Plasma | task-kde-desktop |
| Xfce | task-xfce-desktop |
| LXQt | task-lxqt-desktop |
| MATE | task-mate-desktop |
| LXDE | task-lxde-desktop |

## Synthesis Lifecycle

Upon invocation, Kagami executes the following sequential phases:

1. Prerequisite verification and privilege assessment
2. Directory structure initialisation
3. Base system bootstrap via `debootstrap`
4. Filesystem mounting and chroot preparation
5. System configuration and APT source registration
6. Package installation (essential, kernel, additional)
7. Snapd suppression (Ubuntu only, seven-layer architecture)
8. Desktop environment deployment
9. Flatpak support configuration (optional)
10. Bootloader configuration (GRUB BIOS and EFI)
11. Chroot cleanup and filesystem preparation
12. SquashFS image creation
13. ISO synthesis via xorriso
14. Mount cleanup and workspace finalisation

## Validation and Deployment

### Virtualised Validation

```
qemu-system-x86_64 -cdrom kagami-debian-bookworm.iso -m 2048 -enable-kvm -boot d
```

### Physical Media Writing

```
sudo dd if=<iso> of=/dev/sdX bs=4M status=progress oflag=sync
```

## Snapd Suppression Methodology (Ubuntu)

The seven-layer defensive architecture comprises: APT policy pinning (Priority -1), systemd service masking, pre-installation hook enforcement, binary diversion to null interfaces, environment variable constraints, MOTD status notification, and local filesystem marker verification. This architecture is inapplicable to Debian builds, where snapd is not a system component.
