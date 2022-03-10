package caspio

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/trufflesecurity/trufflehog/v3/pkg/common"
	"github.com/trufflesecurity/trufflehog/v3/pkg/detectors"
	"github.com/trufflesecurity/trufflehog/v3/pkg/pb/detectorspb"
)

type Scanner struct{}

// Ensure the Scanner satisfies the interface at compile time
var _ detectors.Detector = (*Scanner)(nil)

var (
	client = common.SaneHttpClient()

	//Make sure that your group is surrounded in boundry characters such as below to reduce false positives
	keyPat    = regexp.MustCompile(detectors.PrefixRegex([]string{"caspio"}) + `\b([a-z0-9]{50})\b`)
	idPat     = regexp.MustCompile(detectors.PrefixRegex([]string{"caspio"}) + `\b([a-z0-9]{50})\b`)
	domainPat = regexp.MustCompile(detectors.PrefixRegex([]string{"caspio"}) + `\b([a-z0-9]{8})\b`)
)

// Keywords are used for efficiently pre-filtering chunks.
// Use identifiers in the secret preferably, or the provider name.
func (s Scanner) Keywords() []string {
	return []string{"caspio"}
}

// FromData will find and optionally verify Caspio secrets in a given set of bytes.
func (s Scanner) FromData(ctx context.Context, verify bool, data []byte) (results []detectors.Result, err error) {
	dataStr := string(data)

	matches := keyPat.FindAllStringSubmatch(dataStr, -1)
	idMatches := idPat.FindAllStringSubmatch(dataStr, -1)
	domainMatches := domainPat.FindAllStringSubmatch(dataStr, -1)

	for _, match := range matches {
		if len(match) != 2 {
			continue
		}
		resMatch := strings.TrimSpace(match[1])

		for _, idMatch := range idMatches {
			if len(idMatch) != 2 {
				continue
			}

			resIdMatch := strings.TrimSpace(idMatch[1])

			for _, domainMatch := range domainMatches {
				if len(domainMatch) != 2 {
					continue
				}

				resDomainMatch := strings.TrimSpace(domainMatch[1])

				s1 := detectors.Result{
					DetectorType: detectorspb.DetectorType_Caspio,
					Raw:          []byte(resMatch),
				}

				if verify {
					payload := strings.NewReader(fmt.Sprintf(`grant_type=client_credentials&client_id=%s&client_secret=%s`, resIdMatch, resMatch))
					req, _ := http.NewRequest("POST", fmt.Sprintf("https://%s.caspio.com/oauth/token", resDomainMatch), payload)
					req.Header.Add("Content-Type", "text/plain")
					res, err := client.Do(req)
					if err == nil {
						defer res.Body.Close()
						if res.StatusCode >= 200 && res.StatusCode < 300 {
							s1.Verified = true
						} else {
							//This function will check false positives for common test words, but also it will make sure the key appears 'random' enough to be a real key
							if detectors.IsKnownFalsePositive(resMatch, detectors.DefaultFalsePositives, true) {
								continue
							}
						}
					}
				}

				results = append(results, s1)
			}
		}
	}

	return detectors.CleanResults(results), nil
}
