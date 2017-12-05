/*******************************************************************************
*
* Copyright 2017 SAP SE
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You should have received a copy of the License along with this
* program. If not, you may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*
*******************************************************************************/

package objects

import (
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"io"
	"io/ioutil"
	"net/http"
)

//YumSource is a URLSource containing a Yum repository. This type reuses the
//Validate() and Connect() logic of URLSource, but adds a custom scraping
//implementation that reads the Yum repository metadata instead of relying on
//directory listings.
type YumSource URLSource

//Validate implements the Source interface.
func (s *YumSource) Validate(name string) []error {
	return (*URLSource)(s).Validate(name)
}

//Connect implements the Source interface.
func (s *YumSource) Connect() error {
	return (*URLSource)(s).Connect()
}

//ListEntries implements the Source interface.
func (s *YumSource) ListEntries(directoryPath string) ([]FileSpec, *ListEntriesError) {
	return nil, &ListEntriesError{
		Location: (*URLSource)(s).getURLForPath(directoryPath).String(),
		Message:  "ListEntries is not implemented for YumSource",
	}
}

//GetFile implements the Source interface.
func (s *YumSource) GetFile(directoryPath string, targetState FileState) (body io.ReadCloser, sourceState FileState, err error) {
	return (*URLSource)(s).GetFile(directoryPath, targetState)
}

//ListAllFiles implements the Source interface.
func (s *YumSource) ListAllFiles() ([]FileSpec, *ListEntriesError) {
	repomdPath := "repodata/repomd.xml"

	//parse repomd.xml to find paths of all other metadata files
	var repomd struct {
		Entries []struct {
			Type     string `xml:"type,attr"`
			Location struct {
				Href string `xml:"href,attr"`
			} `xml:"location"`
		} `xml:"data"`
	}
	repomdURL, lerr := s.downloadAndParseXML(repomdPath, &repomd)
	if lerr != nil {
		return nil, lerr
	}

	//note metadata files for transfer
	hrefsByType := make(map[string]string)
	allFiles := []FileSpec{
		{Path: repomdPath},
	}
	for _, entry := range repomd.Entries {
		allFiles = append(allFiles, FileSpec{Path: entry.Location.Href})
		hrefsByType[entry.Type] = entry.Location.Href
	}

	//parse primary.xml.gz to find paths of RPMs
	href, exists := hrefsByType["primary"]
	if !exists {
		return nil, &ListEntriesError{
			Location: repomdURL,
			Message:  "cannot find link to primary.xml.gz in repomd.xml",
		}
	}
	var primary struct {
		Packages []struct {
			Location struct {
				Href string `xml:"href,attr"`
			} `xml:"location"`
		} `xml:"package"`
	}
	_, lerr = s.downloadAndParseXML(href, &primary)
	if lerr != nil {
		return nil, lerr
	}
	for _, pkg := range primary.Packages {
		allFiles = append(allFiles, FileSpec{Path: pkg.Location.Href})
	}

	//parse prestodelta.xml.gz (if present) to find paths of DRPMs
	href, exists = hrefsByType["prestodelta"]
	if exists {
		var prestodelta struct {
			Packages []struct {
				Delta struct {
					Href string `xml:"filename"`
				} `xml:"delta"`
			} `xml:"newpackage"`
		}
		_, lerr = s.downloadAndParseXML(href, &prestodelta)
		if lerr != nil {
			return nil, lerr
		}
		for _, pkg := range prestodelta.Packages {
			allFiles = append(allFiles, FileSpec{Path: pkg.Delta.Href})
		}
	}

	//TODO since we downloaded some metadata files already, it would be nice to pass the downloaded contents on to the transfer workers (esp. to ensure consistency with the scraped set of packages)
	return allFiles, nil
}

//Helper function for YumSource.ListAllFiles().
func (s *YumSource) downloadAndParseXML(path string, data interface{}) (uri string, e *ListEntriesError) {
	buf, uri, lerr := s.getFileContents(path)
	if lerr != nil {
		return uri, lerr
	}

	//if `buf` has the magic number for GZip, decompress before parsing as XML
	if bytes.HasPrefix(buf, []byte{0x1f, 0x8b, 0x08}) {
		reader, err := gzip.NewReader(bytes.NewReader(buf))
		if err == nil {
			buf, err = ioutil.ReadAll(reader)
		}
		if err != nil {
			return uri, &ListEntriesError{
				Location: uri,
				Message:  "error while decompressing GZip archive: " + err.Error(),
			}
		}
	}

	err := xml.Unmarshal(buf, data)
	if err != nil {
		return uri, &ListEntriesError{
			Location: uri,
			Message:  "error while parsing XML: " + err.Error(),
		}
	}

	return uri, nil
}

//Helper function for YumSource.ListAllFiles().
func (s *YumSource) getFileContents(path string) (contents []byte, uri string, e *ListEntriesError) {
	u := (*URLSource)(s)
	uri = u.getURLForPath(path).String()

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, uri, &ListEntriesError{uri, "GET failed: " + err.Error()}
	}

	resp, err := u.HTTPClient.Do(req)
	if err != nil {
		return nil, uri, &ListEntriesError{uri, "GET failed: " + err.Error()}
	}
	defer resp.Body.Close()

	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, uri, &ListEntriesError{uri, "GET failed: " + err.Error()}
	}
	return result, uri, nil
}
