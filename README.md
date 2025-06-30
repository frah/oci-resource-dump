# OCI Resource Dump ☁️🔍

OCI Resource Dump is a command-line tool for discovering and listing resources within your Oracle Cloud Infrastructure (OCI) tenancy. Written in Go, it uses instance principal authentication to communicate with OCI APIs.

The primary goal of this tool is to quickly inventory resources in an OCI environment, providing a centralized view of your assets. The output is available in JSON, CSV, and TSV formats, making it easy to integrate with other tools and automation workflows.

## ✨ Features

- 🗺️ **Resource Discovery**: Automatically discovers resources across major OCI services, including compute, networking, storage, and databases.
- 📄 **Flexible Output**: Supports `json` (default), `csv`, and `tsv` formats for easy consumption.
- 🔬 **Advanced Filtering**: Narrow down the discovery scope based on:
    - Compartments (include/exclude by OCID)
    - Resource Types (include/exclude)
    - Resource Names (regex pattern matching)
- 🔄 **Diff Analysis**: Compares two JSON dump files to report added, removed, or modified resources. Ideal for tracking infrastructure changes and for auditing purposes.
- ⚙️ **Configuration File**: Use a `yaml` file to persist your command-line options for consistent runs.
- 🚀 **Performance**: Built for speed in large-scale environments with parallel compartment processing, automatic API error retries, and compartment name caching.
- 📊 **Interactive Progress**: Displays a progress bar with an ETA to monitor the discovery process in real-time.

## 📋 Prerequisites

- Go development environment (version 1.24.4 or later)
- Access to an OCI tenancy
- An OCI compute instance with Instance Principal authentication enabled.
    - The instance must be granted appropriate IAM policies to read the target resources.

## 🛠️ Getting Started

Clone the repository and build the executable:

```bash
git clone https://github.com/your-username/oci-resource-dump.git
cd oci-resource-dump
go build -o oci-resource-dump .
```

## 🚀 Usage

### Basic Resource Discovery

By default, the tool discovers resources in all accessible compartments and prints the output to stdout in JSON format.

```bash
./oci-resource-dump
```

To output to a file in CSV format:

```bash
./oci-resource-dump --format csv --output-file resources.csv
```

### Filtering Example

Target specific compartments and resource types with a name filter:

```bash
./oci-resource-dump \
  --compartments "ocid1.compartment.oc1..prod-compartment-ocid" \
  --resource-types "ComputeInstance,VCN,Subnet" \
  --name-filter "^prod-.*"
```

### Diff Analysis Example

Compare two snapshots of your resources to generate a text report of the changes.

```bash
# 1. Save the initial state
./oci-resource-dump --output-file before.json

# (Make changes to your infrastructure)

# 2. Save the new state
./oci-resource-dump --output-file after.json

# 3. Compare the two states
./oci-resource-dump --compare-files before.json,after.json --diff-format text
```

## ⚙️ Configuration

Instead of passing command-line arguments every time, you can use a configuration file named `oci-resource-dump.yaml`.

Generate a default configuration template with this command:

```bash
./oci-resource-dump --generate-config
```

Edit the generated `oci-resource-dump.yaml` to customize the default behavior.

**Configuration Priority Order:**
1. Command-line arguments (highest)
2. Environment variable (`OCI_DUMP_CONFIG_FILE`)
3. `./oci-resource-dump.yaml` (current directory)
4. `~/.oci-resource-dump.yaml` (home directory)
5. `/etc/oci-resource-dump.yaml` (system directory)
6. Default values (lowest)

## 📦 Supported Resources

This tool can discover the following resource types:

- APIGateway
- AutonomousDatabase
- BlockVolume
- BlockVolumeBackup
- BootVolume
- BootVolumeBackup
- CloudExadataInfrastructure
- ComputeInstance
- DatabaseSystem
- DRG
- ExadataInfrastructure
- FileStorageSystem
- Function
- LoadBalancer
- LocalPeeringGateway
- NetworkLoadBalancer
- ObjectStorageBucket
- OKECluster
- Stream
- Subnet
- VCN

## 📜 License

This project is licensed under the MIT License.
