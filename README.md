# kagami

`kagami` builds a vanilla Ubuntu live ISO image.

> [!WARNING]
> This project is currently not maintained until [ubuntu-vanilla-build](https://github.com/jimedrandatorg/ubuntu-vanilla-build) got toasted and ready to re-implementing Kagami project. Do not use this repo until those project is baked and ready to port into Go.

This project ports the legacy shell pipeline in `references/build.sh` to Go,
while preserving its core flow: host setup, debootstrap, chroot customization,
then hybrid BIOS/UEFI ISO creation.

## Host OS limitation (safety guard)

`kagami` is intentionally restricted to:
- Ubuntu-based systems
- Debian-based systems

On unsupported distributions, the host phase fails early before it runs package
manager or workspace-changing steps. This is to reduce accidental incidents on
systems the pipeline was not designed for.

On pure Debian hosts, ensure Ubuntu archive keys are installed first:

```bash
sudo apt install ubuntu-archive-keyring
```

## How kagami works

`kagami` runs in two phases:

1. **Host phase** (`host`, default):
   - Validates host OS support
   - Installs required host tooling (`debootstrap`, `squashfs-tools`, `xorriso`)
   - Creates a minimal Ubuntu rootfs
   - Enters chroot and runs the chroot phase
   - Produces final ISO + checksum files
2. **Chroot phase** (`chroot`):
   - Configures apt sources
   - Installs base system, kernel, desktop, installer
   - Builds image tree and boot assets
   - Cleans up chroot for ISO packing

Normally you run only the host phase; chroot phase is handled automatically.

## Prerequisites

- Host OS: Ubuntu-based or Debian-based Linux
- Root privileges via `sudo` for build operations
- Go `1.22+` for building from source
- Stable network access for apt/debootstrap/package downloads
- Sufficient free disk space (recommend at least 25-35 GB)

## Build and install

Use the provided `Makefile` helpers.

```bash
# show available targets and variables
make help

# compile local binary
make build

# install to /usr/local/bin/kagami
sudo make install

# remove installed binary
sudo make uninstall
```

`make build` writes the binary directly at `./kagami` (repository root).

Run without installing:

```bash
make run ARGS="--interactive"
```

## Rolling versioning

Builds use a rolling Git-based version string injected at link time:

- format: `rolling.<commit-count>.<short-sha>[-dirty]`
- example: `rolling.128.a1b2c3d4e5f6-dirty`

Useful commands:

```bash
# show resolved version
make version

# build with auto Git version
make build

# override manually if needed
make build VERSION=rolling.custom
```

## Basic usage

General form:

```bash
kagami [flags]
```

### Common workflows

Interactive run (recommended first run):

```bash
kagami --interactive
```

Non-interactive Calamares build:

```bash
kagami \
  --release noble \
  --installer calamares \
  --kernel generic \
  --desktop gnome \
  --name ubuntu-noble \
  --yes
```

Jammy + Ubiquity + XFCE:

```bash
kagami \
  --release jammy \
  --installer ubiquity \
  --kernel lowlatency \
  --desktop xfce \
  --name ubuntu-jammy-xfce \
  --yes
```

Use a custom workspace location:

```bash
kagami --workspace /var/tmp/kagami-work --interactive
```

## Flags

- `--release`: `jammy|noble|resolute`
- `--mirror`: Ubuntu mirror URL (default `http://archive.ubuntu.com/ubuntu/`)
- `--kernel`: `generic|lowlatency`
- `--installer`: `calamares|ubiquity` (`ubiquity` only for `jammy`)
- `--desktop`: `gnome|xfce`
- `--gnome-recommends`: `0|1` (only relevant when `--desktop gnome`)
- `--name`: output ISO base name (`<name>.iso`)
- `--liveboot-label`: GRUB live-boot menu label
- `--workspace`: workspace directory
- `--references-dir`: references assets path (default `./references`)
- `--yes`: skip confirmation prompt
- `--interactive` / `-i`: force interactive selection
- `--phase`: `host|chroot` (advanced/debug use)

## Environment variables

Flags can be supplied by environment defaults:

- `TARGET_UBUNTU_VERSION`
- `TARGET_UBUNTU_MIRROR`
- `TARGET_KERNEL_FLAVOR`
- `TARGET_INSTALLER`
- `TARGET_DESKTOP`
- `TARGET_GNOME_INSTALL_RECOMMENDS`
- `TARGET_NAME`
- `GRUB_LIVEBOOT_LABEL`
- `KAGAMI_WORKSPACE` (preferred)
- `UBUNTU_VANILLA_WORKSPACE` (legacy, deprecated)
- `KAGAMI_PHASE`

## Outputs and paths

By default, generated artifacts are written in the repository root:

- `<name>.iso`
- `<name>.iso.sha1`
- `<name>.iso.sha256`

Workspace defaults to `<repo>/workspace` and is cleaned during/after build.
On WSL/DrvFs-like filesystems, kagami can relocate workspace to a Linux-native
path for reliability.

## Safety and operational notes

- The build process modifies packages and filesystem trees under privileged
  contexts (host + chroot).
- Build interruptions can leave partial workspace state if the process is
  externally terminated.
- Prefer running one build at a time per workspace path.
- Review release/installer compatibility before running non-interactively:
  - `ubiquity` is only supported on `jammy`
  - `calamares` works on all supported releases

## Troubleshooting

- **Unsupported host OS**: run on an Ubuntu-based or Debian-based system.
- **Debian keyring error**: install `ubuntu-archive-keyring`.
- **Permission errors**: ensure `sudo` is available and your user can run it.
- **Network/mirror failures**: verify mirror reachability or set `--mirror`.
