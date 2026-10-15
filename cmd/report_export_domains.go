package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/sensepost/gowitness/internal/ascii"
	"github.com/sensepost/gowitness/internal/islazy"
	"github.com/sensepost/gowitness/pkg/database"
	"github.com/sensepost/gowitness/pkg/log"
	"github.com/sensepost/gowitness/pkg/models"
	"github.com/spf13/cobra"
	"gorm.io/gorm/clause"
)

var exportDomainsCmdFlags = struct {
	DbURI      string
	JsonFile   string
	Technology string
	OutputFile string
}{}

var exportDomainsCmd = &cobra.Command{
	Use:   "export-domains",
	Short: "Export unique domains for a technology to a text file",
	Long: ascii.LogoHelp(ascii.Markdown(`
# report export-domains

Export unique domains for a selected technology into a newline delimited text
file so they can be reused in other tooling.

The data source can be a database URI (default) or a JSON Lines file. If both
are set, the JSON file takes precedence.
`)),
	Example: ascii.Markdown(`
- gowitness report export-domains --technology nginx --output-file nginx-domains.txt
- gowitness report export-domains --technology wordpress --db-uri sqlite://gowitness.sqlite3
- gowitness report export-domains --technology jenkins --json-file gowitness.jsonl --output-file jenkins.txt`),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(exportDomainsCmdFlags.Technology) == "" {
			return errors.New("a technology must be specified with --technology")
		}

		if strings.TrimSpace(exportDomainsCmdFlags.OutputFile) == "" {
			return errors.New("an output file must be specified with --output-file")
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		technology := strings.TrimSpace(exportDomainsCmdFlags.Technology)

		var results []*models.Result
		var err error

		if exportDomainsCmdFlags.JsonFile != "" {
			results, err = exportDomainsResultsFromJSONL(exportDomainsCmdFlags.JsonFile, technology)
			if err != nil {
				log.Error("could not read results from JSON Lines file", "err", err)
				return
			}
		} else {
			results, err = exportDomainsResultsFromDB(exportDomainsCmdFlags.DbURI, technology)
			if err != nil {
				log.Error("could not read results from database", "err", err)
				return
			}
		}

		domains := uniqueDomains(results)
		if len(domains) == 0 {
			log.Warn("no matching domains found for technology", "technology", technology)
			return
		}

		outputPath, err := islazy.CreateFileWithDir(exportDomainsCmdFlags.OutputFile)
		if err != nil {
			log.Error("could not create output file", "err", err)
			return
		}

		if err := writeDomains(outputPath, domains); err != nil {
			log.Error("could not write domains to output file", "err", err)
			return
		}

		log.Info("export completed", "technology", technology, "total-domains", len(domains), "output-file", outputPath)
	},
}

func init() {
	reportCmd.AddCommand(exportDomainsCmd)

	exportDomainsCmd.Flags().StringVar(&exportDomainsCmdFlags.DbURI, "db-uri", "sqlite://gowitness.sqlite3", "The location of a gowitness database")
	exportDomainsCmd.Flags().StringVar(&exportDomainsCmdFlags.JsonFile, "json-file", "", "The location of a JSON Lines results file (e.g., ./gowitness.jsonl). This flag takes precedence over --db-uri")
	exportDomainsCmd.Flags().StringVar(&exportDomainsCmdFlags.Technology, "technology", "", "The technology to filter for (case-insensitive), for example nginx")
	exportDomainsCmd.Flags().StringVar(&exportDomainsCmdFlags.OutputFile, "output-file", "domains.txt", "The output text file where matching domains will be written")
}

func exportDomainsResultsFromDB(dbURI, technology string) ([]*models.Result, error) {
	conn, err := database.Connection(dbURI, true, false)
	if err != nil {
		return nil, err
	}

	var results []*models.Result
	err = conn.Model(&models.Result{}).
		Joins("JOIN technologies ON technologies.result_id = results.id").
		Where("LOWER(technologies.value) = LOWER(?)", technology).
		Preload(clause.Associations).
		Find(&results).Error

	if err != nil {
		return nil, err
	}

	return results, nil
}

func exportDomainsResultsFromJSONL(jsonPath, technology string) ([]*models.Result, error) {
	file, err := os.Open(jsonPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var results []*models.Result
	reader := bufio.NewReader(file)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				if len(line) == 0 {
					break
				}
			} else {
				return nil, err
			}
		}

		var result models.Result
		if err := json.Unmarshal(line, &result); err != nil {
			log.Error("could not unmarshal JSON line", "err", err)
			if err == io.EOF {
				break
			}
			continue
		}

		if resultHasTechnology(&result, technology) {
			results = append(results, &result)
		}

		if err == io.EOF {
			break
		}
	}

	return results, nil
}

func resultHasTechnology(result *models.Result, technology string) bool {
	for _, tech := range result.Technologies {
		if strings.EqualFold(strings.TrimSpace(tech.Value), strings.TrimSpace(technology)) {
			return true
		}
	}

	return false
}

func uniqueDomains(results []*models.Result) []string {
	seen := make(map[string]bool)
	domains := make([]string, 0)

	for _, result := range results {
		if result == nil {
			continue
		}

		domain := ""
		if result.FinalURL != "" {
			domain = domainFromURL(result.FinalURL)
		}
		if domain == "" {
			domain = domainFromURL(result.URL)
		}

		if domain == "" || seen[domain] {
			continue
		}

		seen[domain] = true
		domains = append(domains, domain)
	}

	sort.Strings(domains)

	return domains
}

func domainFromURL(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}

	parsed, err := url.ParseRequestURI(target)
	if err != nil {
		return ""
	}

	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return ""
	}

	return strings.ToLower(host)
}

func writeDomains(filePath string, domains []string) error {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	for _, domain := range domains {
		if _, err := writer.WriteString(fmt.Sprintf("%s\n", domain)); err != nil {
			return err
		}
	}

	return nil
}
