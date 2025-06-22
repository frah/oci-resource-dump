package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
)

// outputJSON outputs resources in JSON format with pretty printing
func outputJSON(resources []ResourceInfo) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(resources)
}

// outputCSV outputs resources in CSV format with headers
func outputCSV(resources []ResourceInfo) error {
	writer := csv.NewWriter(os.Stdout)
	defer writer.Flush()

	// Write header
	header := []string{"ResourceType", "ResourceName", "OCID", "CompartmentID", "AdditionalInfo"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, resource := range resources {
		additionalInfoJSON, _ := json.Marshal(resource.AdditionalInfo)
		record := []string{
			resource.ResourceType,
			resource.ResourceName,
			resource.OCID,
			resource.CompartmentID,
			string(additionalInfoJSON),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// outputTSV outputs resources in TSV (Tab-Separated Values) format
func outputTSV(resources []ResourceInfo) error {
	// Write header
	fmt.Println("ResourceType\tResourceName\tOCID\tCompartmentID\tAdditionalInfo")

	// Write data
	for _, resource := range resources {
		additionalInfoJSON, _ := json.Marshal(resource.AdditionalInfo)
		fmt.Printf("%s\t%s\t%s\t%s\t%s\n",
			resource.ResourceType,
			resource.ResourceName,
			resource.OCID,
			resource.CompartmentID,
			string(additionalInfoJSON),
		)
	}

	return nil
}

// outputResources routes output to the appropriate format function (stdout)
func outputResources(resources []ResourceInfo, format string) error {
	switch format {
	case "json":
		return outputJSON(resources)
	case "csv":
		return outputCSV(resources)
	case "tsv":
		return outputTSV(resources)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// outputResourcesToFile outputs resources to a file in the specified format
func outputResourcesToFile(resources []ResourceInfo, format, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	switch format {
	case "json":
		return outputJSONToFile(resources, file)
	case "csv":
		return outputCSVToFile(resources, file)
	case "tsv":
		return outputTSVToFile(resources, file)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// outputJSONToFile outputs resources in JSON format to a file
func outputJSONToFile(resources []ResourceInfo, file *os.File) error {
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(resources)
}

// outputCSVToFile outputs resources in CSV format to a file
func outputCSVToFile(resources []ResourceInfo, file *os.File) error {
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"ResourceType", "ResourceName", "OCID", "CompartmentID", "AdditionalInfo"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, resource := range resources {
		additionalInfoJSON, _ := json.Marshal(resource.AdditionalInfo)
		record := []string{
			resource.ResourceType,
			resource.ResourceName,
			resource.OCID,
			resource.CompartmentID,
			string(additionalInfoJSON),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// outputTSVToFile outputs resources in TSV format to a file
func outputTSVToFile(resources []ResourceInfo, file *os.File) error {
	// Write header
	if _, err := fmt.Fprintln(file, "ResourceType\tResourceName\tOCID\tCompartmentID\tAdditionalInfo"); err != nil {
		return err
	}

	// Write data
	for _, resource := range resources {
		additionalInfoJSON, _ := json.Marshal(resource.AdditionalInfo)
		if _, err := fmt.Fprintf(file, "%s\t%s\t%s\t%s\t%s\n",
			resource.ResourceType,
			resource.ResourceName,
			resource.OCID,
			resource.CompartmentID,
			string(additionalInfoJSON),
		); err != nil {
			return err
		}
	}

	return nil
}