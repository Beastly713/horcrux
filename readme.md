# Horcrux

> "I am not worried, Harry," said Dumbledore, his voice a little stronger despite the freezing water. "I am with you."

**Horcrux** is a secure, open-source tool written in Go that allows you to split your sensitive files into encrypted fragments (horcruxes). To restore the original file, you only need a specific subset (threshold) of these fragments.

It combines **AES-256-GCM encryption**, **Shamir’s Secret Sharing**, and **Reed-Solomon erasure coding** to ensure your data is both secure and resilient against loss. It also supports **Steganography**, allowing you to hide your encrypted shards inside seemingly innocent images.


## Key Features

* **Threshold Recovery**: Split a file into `N` parts, requiring only `T` parts to recover it (e.g., "3 of 5").
* **Strong Encryption**: Uses **AES-256-GCM** with ephemeral keys for authenticated encryption.
* **Shamir's Secret Sharing**: The encryption key itself is cryptographically split; no single shard holds the full key.
* **Erasure Coding**: Uses **Reed-Solomon** to split the encrypted payload, offering resilience against data corruption.
* **Steganography**: Optionally hide shards inside **PNG images** using LSB encoding.
* **Paranoiac Mode**: Remove all metadata headers for maximum obscurity (requires manual tracking of file order/threshold).
* **Interactive TUI**: A beautiful terminal UI for easily managing and binding your horcruxes.
* **Compression**: Automatic Gzip compression to minimize storage footprint.

## Installation

Ensure you have **Go 1.25+** installed.

```bash
# Clone the repository
git clone [https://github.com/Beastly713/horcrux.git](https://github.com/Beastly713/horcrux.git)

# Navigate to the directory
cd horcrux

# Build the binary
go build -o horcrux main.go

# (Optional) Install to your $GOBIN
go install
```
## Usage

## 1. Split a File
Split a file into multiple fragments. You must specify the total number of shards (`-n`) and the threshold required to recover (`-t`).
```bash
# Basic usage: Create 5 shards, requiring 3 to restore
./horcrux split secret_diary.txt -n 5 -t 3

# Output to a specific directory
./horcrux split sensitive.pdf -n 7 -t 4 -d ./safe_storage
```
## Flags:
- `-n`, `--shards`: Total number of horcruxes to generate (Required).
- `-t`, `--threshold`: Number of horcruxes required to resurrect the file (Required).
- `-d`, `--destination`: Output directory (default: current directory).
- `-i`, `--carrier-image`: Path to an image (PNG/JPG) to hide data inside.
- `--headerless`: Enable "Paranoiac mode" (no metadata/headers).

## 2. Bind (Resurrect) a File
Restore the original file by pointing the tool at a directory containing the required number of `.horcrux` (or `.png`) files.
```bash
# Restore a file from the current directory
./horcrux bind .

# Restore from a specific folder to a specific destination
./horcrux bind ./my_shards --destination ./restored_files
```

### Flags:
- `-d`, `--destination`: Directory to write the resurrected file.
- `--overwrite`: Overwrite the file if it already exists.

## 3. Interactive Mode (TUI)
Launch a terminal UI to browse files and select specific shards to bind.
```bash
./horcrux interactive
```
- Navigation: ↑ / ↓
- Select: Space
- Bind: b
- Quit: q 
 
## Steganography Support 
You can hide your encrypted shards inside images so they appear as normal picture files.
Provide a carrier image (e.g., vacation.jpg).
Horcrux will create copies of this image (e.g., vacation_1_of_5.png) with the data embedded in the pixels.
the output is always PNG to prevent data loss from compression artifacts.
```bash 
./horcrux split nuclear_codes.txt - n 3 - t 2 --carrier-image cat_photo.jpg 
```
To restore, simply have the PNGs in the directory and run bind. The tool automatically detects hidden data.

## How It Works
### Compression
- The input file is compressed using Gzip.

### Encryption
- A random 32-byte ephemeral key is generated.
- The data is encrypted using AES-256-GCM (Authenticated Encryption).

### Key Splitting
- The ephemeral key is split into N fragments using Shamir's Secret Sharing.

### Payload Sharding
- The encrypted binary is split into N shards using Reed-Solomon erasure coding.

### Packaging
- Each output file contains one Key Fragment and one Data Shard.
- Unless using `--headerless`, a JSON header is added containing the file index and reconstruction metadata.
 