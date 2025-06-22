# Config Module Design Document

## Phase 2A: Basic Configuration File Support

### Objective
Implement YAML configuration file support for existing CLI features only, maintaining full backward compatibility.

### Current CLI Arguments (Phase 2A Scope)
```bash
--timeout, -t        int     (300)     # Timeout in seconds
--log-level, -l      string  (normal)  # Log level
--format, -f         string  (json)    # Output format  
--progress           bool    (false)   # Show progress
--no-progress        bool    (false)   # Disable progress
```

### YAML Structure (Phase 2A)
```yaml
version: "1.0"
general:
  timeout: 300
  log_level: "normal"
  output_format: "json"
  progress: true
output:
  file: ""              # New feature: file output
```

### Go Structures (config.go)
```go
type AppConfig struct {
    Version string        `yaml:"version"`
    General GeneralConfig `yaml:"general"`
    Output  OutputConfig  `yaml:"output"`
}

type GeneralConfig struct {
    Timeout      int    `yaml:"timeout"`
    LogLevel     string `yaml:"log_level"`
    OutputFormat string `yaml:"output_format"`
    Progress     bool   `yaml:"progress"`
}

type OutputConfig struct {
    File string `yaml:"file"`
}
```

### Priority Order
1. CLI arguments (highest priority)
2. Environment variable: OCI_DUMP_CONFIG_FILE
3. ./oci-resource-dump.yaml (current directory)
4. ~/.oci-resource-dump.yaml (home directory)
5. /etc/oci-resource-dump.yaml (system)
6. Default values (lowest priority)

### Integration Points
- main.go: CLI argument parsing integration
- types.go: Config struct extension
- New: config.go module creation

### Phase 2B Extensions (Future)
```yaml
# Will be added in Phase 2B
filters:
  include_compartments: []
  exclude_compartments: []
  include_resource_types: []
  exclude_resource_types: []
  name_pattern: ""
```

### Phase 2C Extensions (Future)  
```yaml
# Will be added in Phase 2C
diff:
  enabled: false
  format: "text"
```

### File Locations
- `config.go`: New configuration module
- `oci-resource-dump.yaml.example`: Sample configuration
- Modified: `main.go`, `types.go`

### Dependencies
- gopkg.in/yaml.v3: YAML parsing library