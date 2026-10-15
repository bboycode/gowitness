package api

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/sensepost/gowitness/pkg/log"
	"github.com/sensepost/gowitness/pkg/models"
)

var exportFileNameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// ExportDomainsHandler exports unique domains for a specific technology.
//
//	@Summary		Export domains by technology
//	@Description	Export unique hostnames for a selected technology as newline-delimited text.
//	@Tags			Results
//	@Accept			json
//	@Produce		plain
//	@Param			technology	query		string	true	"The technology to filter for"
//	@Success		200			{string}	string	"newline-delimited domains"
//	@Router			/results/export/domains [get]
func (h *ApiHandler) ExportDomainsHandler(w http.ResponseWriter, r *http.Request) {
	technology := strings.TrimSpace(r.URL.Query().Get("technology"))
	if technology == "" {
		http.Error(w, "technology query parameter is required", http.StatusBadRequest)
		return
	}

	var results []models.Result
	if err := h.DB.Model(&models.Result{}).
		Joins("JOIN technologies ON technologies.result_id = results.id").
		Where("LOWER(technologies.value) = LOWER(?)", technology).
		Find(&results).Error; err != nil {

		log.Error("failed querying results for export", "technology", technology, "err", err)
		http.Error(w, "failed querying results", http.StatusInternalServerError)
		return
	}

	domains := uniqueDomainsFromResults(results)

	filename := fmt.Sprintf("%s-domains.txt", sanitizeExportFileName(technology))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	writer := bufio.NewWriter(w)
	for _, domain := range domains {
		if _, err := writer.WriteString(domain + "\n"); err != nil {
			log.Error("failed writing export response", "err", err)
			return
		}
	}

	if err := writer.Flush(); err != nil {
		log.Error("failed flushing export response", "err", err)
	}
}

func uniqueDomainsFromResults(results []models.Result) []string {
	seen := make(map[string]bool)
	domains := make([]string, 0, len(results))

	for _, result := range results {
		domain := ""
		if result.FinalURL != "" {
			domain = domainFromTargetURL(result.FinalURL)
		}
		if domain == "" {
			domain = domainFromTargetURL(result.URL)
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

func domainFromTargetURL(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}

	parsed, err := url.ParseRequestURI(target)
	if err != nil {
		return ""
	}

	hostname := strings.TrimSpace(parsed.Hostname())
	if hostname == "" {
		return ""
	}

	return strings.ToLower(hostname)
}

func sanitizeExportFileName(value string) string {
	cleaned := strings.TrimSpace(strings.ToLower(value))
	cleaned = exportFileNameSanitizer.ReplaceAllString(cleaned, "-")
	cleaned = strings.Trim(cleaned, "-._")

	if cleaned == "" {
		return "technology"
	}

	return cleaned
}
