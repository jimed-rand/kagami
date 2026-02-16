# Kagami: A Comprehensive System for Automated Ubuntu ISO Synthesis

## Abstract

Kagami is a specialized orchestration framework designed for the deterministic synthesis of Ubuntu-based operating system images. The system prioritizes the deployment of vanilla desktop environments and the implementation of immutable restrictions on the snapd package management daemon. By utilizing a modular configuration-driven approach, Kagami enables the generation of streamlined, high-performance distributions tailored to specific computational requirements.

## Etymology and Nomenclature

The designation "Kagami" is derived from the Japanese term for "mirror" (kagami). This nomenclature serves a dual purpose:

1. **Philosophical Reflection**: The system is designed to provide a direct and undistorted reflection of the user's specified configuration within the resulting installation medium. 
2. **Cultural Reference**: The project acknowledges an inspiration from the character Kagami Hiiragi of the anime series "Lucky Star," reflecting the developer's cultural influences and the project's identity.

## Functional Domain and Features

Kagami facilitates the following technical objectives:

- **Vanilla Desktop Environment Proliferation**: Support for nine distinct desktop environments, including GNOME, KDE Plasma, XFCE, and others, emphasizing core component installation without extraneous software bloat.
- **Permanent Snapd Constraint**: Implementation of a seven-layer defensive architecture to prevent the initialization or execution of the snapd service.
- **Automated Dependency Resolution**: Recursive verification and installation of required system utilities to ensure operational parity.
- **Cross-Architecture Compatibility**: Provision of support for both UEFI and BIOS boot protocols within the x86_64 architecture.

## System Prerequisites

### Distribution Support

Kagami is optimized for execution on Advanced Package Tool (APT) based environments:
- Ubuntu (LTS and current interim releases)
- Debian (Stable and testing branches)
- Derivative distributions (e.g., Linux Mint, Elementary OS)

**Note**: Non-APT distributions (e.g., Fedora, Arch, openSUSE) and APT-RPM based systems (e.g., ALT Linux) are explicitly unsupported.

### Computational Requirements

- **Memory**: Minimum of 4GB allocated RAM; 8GB recommended for concurrent build processes.
- **Storage**: Minimum of 15GB of unallocated disk space for temporary chroot environments and final image synthesis.
- **Privileges**: Administrative (root) access is mandatory for filesystem manipulation and package management.

## Methodological Installation

### Source-Based Implementation

```bash
git clone https://github.com/jimed-rand/kagami.git
cd kagami
make build
sudo make install
```

### Dependency Procurement

Users must verify environmental compatibility and procure necessary utilities:

```bash
make check-prereqs
sudo make install-deps
```

Required utilities include `debootstrap`, `squashfs-tools`, `xorriso`, `grub-pc-bin`, `grub-efi-amd64-bin`, `mtools`, `dosfstools`, `isolinux`, and `syslinux`.

## Operational Usage

### 60-Second Quick Start

The following sequence facilitates immediate image generation:
1. `make build`
2. `sudo make install-deps`
3. `sudo ./kagami`

### Configuration Execution

Standard ISO generation is initiated via the following command:

```bash
sudo ./kagami -config <path-to-configuration-json>
```

### Argument Specification

- `-config`: Path to the JSON configuration file.
- `-release`: Specifies the target Ubuntu release (e.g., noble, jammy, focal, resolute, or devel).
- `-hostname`: Designates the network identifier for the target system.
- `-output`: Defines the absolute path for the synthesized ISO file.
- `-workdir`: Specifies the working directory (defaults to the configuration file's directory if provided).

## Comparative Analysis of Vanilla Desktop Environments

Vanilla configurations prioritize core architectural components, minimizing software redundancy.

| Desktop Environment | Display Manager | Memory Usage (Idle) | Storage Allocation |
|---------------------|-----------------|---------------------|-------------------|
| GNOME | GDM3 | ~800MB | ~3.0GB |
| KDE Plasma | SDDM | ~600MB | ~2.5GB |
| XFCE | LightDM | ~400MB | ~1.5GB |
| LXQt | SDDM | ~300MB | ~1.0GB |
| MATE | LightDM | ~400MB | ~1.5GB |

Detailed configuration profiles are available for GNOME Shell, KDE Plasma, XFCE, LXQt, MATE, Budgie, Cinnamon, Unity, and UKUI within the `examples/` directory.

## Synthesis Lifecycle

Upon invocation, Kagami executes the following sequential phases:
1. **Environmental Verification**: Assessment of prerequisites and privileges.
2. **Directory Initialization**: Allocation of workspace resources.
3. **Bootstrap Phase**: Initialization of the core Ubuntu root filesystem.
4. **Environment Encapsulation**: Mounting and configuration of the chroot environment.
5. **Package Proliferation**: Installation of specified software components.
6. **Constraint Implementation**: Permanent suppression of the snapd daemon.
7. **Desktop Deployment**: Integration of the selected graphical interface.
8. **Bootloader Configuration**: Initialization of GRUB protocols.
9. **Finalization**: Creation of the SquashFS image and ISO synthesis.

## Validation and Deployment

### Virtualized Validation

Synthesized images should be validated using hypervisors:

```bash
# QEMU Execution
qemu-system-x86_64 -cdrom kagami-ubuntu-noble.iso -m 2048 -enable-kvm -boot d
```

### Physical Media Synthesis

For hardware initialization, the `dd` utility is recommended for bit-for-bit synthesis to physical media:

```bash
sudo dd if=<iso> of=/dev/sdX bs=4M status=progress oflag=sync
```

## Snapd Suppression Methodology

The system employs a seven-layer defensive architecture:
1. APT policy pinning (Priority -1).
2. Systemd service masking.
3. Pre-installation hooks for package blocking.
4. Binary diversion to null interfaces.
5. Environment variable constraints.
6. Message of the Day (MOTD) status notification.
7. Local filesystem marker verification.

## Conclusion

Kagami provides a rigorous, reproducible methodology for the creation of minimal Ubuntu distributions. By eliminating non-essential components and ensuring a direct mapping from configuration to implementation, it serves as an essential tool for system administrators and power users seeking ultimate control over their operating environment.

---

Kagami Version 4.0 - Reflecting Pure Ubuntu Principles
