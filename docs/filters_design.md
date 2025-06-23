# Filtering Module Design Document

## Phase 2B: Resource Filtering Implementation

### Objective
Implement comprehensive filtering capabilities to improve performance and usability in large-scale OCI environments by allowing users to selectively process specific compartments, resource types, and resources matching name patterns.

### Filtering Categories

#### 1. Compartment Filtering
- **Include Filter**: Process only specified compartments
- **Exclude Filter**: Skip specified compartments  
- **Target**: Reduce API calls by limiting compartment scope

#### 2. Resource Type Filtering
- **Include Filter**: Process only specified resource types
- **Exclude Filter**: Skip specified resource types
- **Target**: Reduce processing time by avoiding expensive resource types

#### 3. Name Pattern Filtering
- **Include Filter**: Process only resources matching regex pattern
- **Exclude Filter**: Skip resources matching regex pattern
- **Target**: Focus on specific naming conventions

### CLI Arguments Design

#### New Arguments
```bash
# Compartment Filtering
--compartments string           # Comma-separated compartment OCIDs to include
--exclude-compartments string   # Comma-separated compartment OCIDs to exclude

# Resource Type Filtering  
--resource-types string         # Comma-separated resource types to include
--exclude-resource-types string # Comma-separated resource types to exclude

# Name Pattern Filtering
--name-filter string           # Regex pattern for resource names to include
--exclude-name-filter string   # Regex pattern for resource names to exclude
```

#### Usage Examples
```bash
# Production environment only
./oci-resource-dump --compartments "ocid1.compartment.oc1..prod" --name-filter "^prod-.*"

# Core infrastructure only
./oci-resource-dump --resource-types "compute_instances,vcns,subnets"

# Exclude test resources
./oci-resource-dump --exclude-name-filter "test-.*|dev-.*"

# Combined filtering
./oci-resource-dump \
  --compartments "ocid1.compartment.oc1..prod,ocid1.compartment.oc1..staging" \
  --resource-types "compute_instances,vcns" \
  --name-filter "^(prod|staging)-.*"
```

### Configuration File Extension

#### Extended YAML Structure
```yaml
version: "1.0"

general:
  timeout: 300
  log_level: "normal"
  output_format: "json"
  progress: true

output:
  file: ""

# New section for Phase 2B
filters:
  include_compartments: []      # List of compartment OCIDs to include
  exclude_compartments: []      # List of compartment OCIDs to exclude
  include_resource_types: []    # List of resource types to include
  exclude_resource_types: []    # List of resource types to exclude
  name_pattern: ""             # Regex pattern for resource names to include
  exclude_name_pattern: ""     # Regex pattern for resource names to exclude
```

#### Example Configuration
```yaml
filters:
  include_compartments:
    - "ocid1.compartment.oc1..aaaaaaaa"
    - "ocid1.compartment.oc1..bbbbbbbb"
  exclude_compartments:
    - "ocid1.compartment.oc1..cccccccc"
  include_resource_types:
    - "compute_instances"
    - "vcns" 
    - "subnets"
  exclude_resource_types:
    - "object_storage_buckets"
    - "streaming"
  name_pattern: "^prod-.*"
  exclude_name_pattern: "test-.*|dev-.*"
```

### Go Structures Design

#### FilterConfig Structure
```go
// filters.go - New module
type FilterConfig struct {
    IncludeCompartments   []string `yaml:"include_compartments"`
    ExcludeCompartments   []string `yaml:"exclude_compartments"`
    IncludeResourceTypes  []string `yaml:"include_resource_types"`
    ExcludeResourceTypes  []string `yaml:"exclude_resource_types"`
    NamePattern          string   `yaml:"name_pattern"`
    ExcludeNamePattern   string   `yaml:"exclude_name_pattern"`
}

// AppConfig extension in config.go
type AppConfig struct {
    Version string        `yaml:"version"`
    General GeneralConfig `yaml:"general"`
    Output  OutputConfig  `yaml:"output"`
    Filters FilterConfig  `yaml:"filters"`  // New section
}
```

### Implementation Strategy

#### Phase 2B-2: filters.go Module Creation
```go
// Core filtering functions
func ApplyCompartmentFilter(compartments []identity.Compartment, filter FilterConfig) []identity.Compartment
func ApplyResourceTypeFilter(resourceType string, filter FilterConfig) bool
func ApplyNameFilter(resourceName string, filter FilterConfig) bool
func ValidateFilterConfig(filter FilterConfig) error
```

#### Phase 2B-3: Compartment & Resource Type Filters
- **Early Filtering**: Apply filters before API calls to reduce processing
- **Whitelist/Blacklist Logic**: Include filters take precedence over exclude filters
- **Performance Optimization**: Skip entire compartments or resource types early

#### Phase 2B-4: Name Pattern Filters  
- **Regex Support**: Full regular expression pattern matching
- **Case Sensitivity**: Case-sensitive matching by default
- **Performance**: Compiled regex patterns for efficiency

#### Phase 2B-5: Configuration Integration
- **Config File Extension**: Add FilterConfig to AppConfig
- **CLI Integration**: New command-line arguments
- **Merge Logic**: CLI arguments override configuration file

#### Phase 2B-6: Discovery Integration
- **Early Exit**: Apply compartment filters before processing
- **Resource Type Skip**: Skip entire resource type discovery
- **Name Filtering**: Apply to individual resources post-discovery

### Resource Type Mapping

#### Supported Resource Types (from discovery.go)
```go
var supportedResourceTypes = []string{
    "ComputeInstances",      // Compute Instances
    "VCNs",                  // Virtual Cloud Networks  
    "Subnets",               // Subnets
    "BlockVolumes",          // Block Volumes
    "ObjectStorageBuckets",  // Object Storage Buckets
    "OKEClusters",           // OKE Clusters
    "LoadBalancers",         // Load Balancers
    "DatabaseSystems",       // Database Systems
    "DRGs",                  // Dynamic Routing Gateways
    "AutonomousDatabases",   // Autonomous Databases
    "Functions",             // Functions
    "APIGateways",           // API Gateways
    "FileStorageSystems",    // File Storage Systems
    "NetworkLoadBalancers",  // Network Load Balancers
    "Streams",               // Streaming Service
}

// CLI-friendly aliases (lowercase with underscores)
var resourceTypeAliases = map[string]string{
    "compute_instances":        "ComputeInstances",
    "vcns":                    "VCNs",
    "subnets":                 "Subnets", 
    "block_volumes":           "BlockVolumes",
    "object_storage_buckets":  "ObjectStorageBuckets",
    "oke_clusters":            "OKEClusters",
    "load_balancers":          "LoadBalancers",
    "database_systems":        "DatabaseSystems",
    "drgs":                    "DRGs",
    "autonomous_databases":    "AutonomousDatabases",
    "functions":               "Functions",
    "api_gateways":            "APIGateways",
    "file_storage_systems":    "FileStorageSystems",
    "network_load_balancers":  "NetworkLoadBalancers",
    "streams":                 "Streams",
}
```

### Performance Impact Analysis

#### Expected Performance Improvements
- **Compartment Filtering**: 50-80% reduction in API calls
- **Resource Type Filtering**: 30-70% reduction in processing time
- **Name Filtering**: 10-30% reduction in output processing

#### Benchmarking Targets
- **Large Environment (100+ compartments)**: < 30% of original execution time
- **Focused Discovery (3 resource types)**: < 20% of original execution time
- **Pattern Matching**: < 5% overhead for regex processing

### Error Handling

#### Validation Rules
- **Compartment OCIDs**: Valid OCID format validation
- **Resource Types**: Must be from supported list
- **Regex Patterns**: Valid regular expression syntax

#### Error Messages
```go
// Invalid compartment OCID
"invalid compartment OCID format: %s"

// Unknown resource type
"unknown resource type '%s', supported types: %v"

// Invalid regex pattern
"invalid regex pattern '%s': %v"

// Conflicting filters
"include and exclude filters cannot both be empty"
```

### Integration Points

#### Modified Files
- `config.go`: FilterConfig integration
- `main.go`: CLI argument parsing for filters
- `discovery.go`: Filter application in discovery process
- `oci-resource-dump.yaml.example`: Updated sample configuration

#### New Files
- `filters.go`: Core filtering logic implementation
- `filters_design.md`: This design document

### Testing Strategy

#### Unit Tests
- Compartment filtering logic
- Resource type filtering logic
- Name pattern filtering (regex)
- Configuration validation

#### Integration Tests
- CLI argument parsing with filters
- Configuration file loading with filters
- End-to-end filtering workflow

#### Performance Tests
- Large compartment list filtering
- Regex pattern matching performance
- Resource type selection efficiency

### Future Enhancements (Post Phase 2B)

#### Tag-Based Filtering (Phase 3)
```yaml
filters:
  tags:
    - key: "Environment"
      value: "Production"
      operator: "equals"
```

#### Advanced Pattern Matching
- Case-insensitive patterns
- Multiple pattern support
- Pattern exclusion priorities

#### Dynamic Filtering
- Interactive filter selection
- Filter recommendation based on environment analysis

### Backward Compatibility

#### Guarantees
- **Existing CLI Arguments**: No changes to current arguments
- **Configuration Files**: Optional filters section
- **Default Behavior**: No filtering applied by default (process all resources)

#### Migration Path
- **Gradual Adoption**: Users can adopt filtering incrementally
- **Documentation**: Clear examples for common use cases
- **Configuration Templates**: Pre-built configurations for common scenarios