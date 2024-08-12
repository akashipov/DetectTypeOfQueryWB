package saver

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/akashipov/DetectTypeOfQueryWB/config"
	errorCustom "github.com/akashipov/DetectTypeOfQueryWB/error"
	"github.com/akashipov/DetectTypeOfQueryWB/logger"
)

type QueryCategory string

const (
	preset          QueryCategory = "Preset"
	extendSearch    QueryCategory = "ExtendSearch"
	merger          QueryCategory = "Merger"
	unknown         QueryCategory = "Unknown"
	presetsFileName               = "list.presets"
	queriesFileName               = "queries.csv"
)

var allExistedValidTypes = []QueryCategory{preset, extendSearch, merger}

type queryInfo struct {
	text        string
	category    QueryCategory
	filterValue string
	presets     []string
}

var unknownQueryInfo = queryInfo{"", unknown, "", nil}

type ExactMatchResponse struct {
	Metadata struct {
		Name         string `json:"name"`
		CatalogValue string `json:"catalog_value"`
	} `json:"metadata"`
}

type Saver struct {
	cfg                  *config.Config
	responseBodies       chan []byte
	categoryToWriter     map[QueryCategory]*bufio.Writer
	categoryToSetPresets map[QueryCategory]map[string]struct{}
	files                []*os.File
	done                 chan struct{}
}

func closeFiles(files []*os.File) (err error) {
	for i := range files {
		err = errors.Join(files[i].Close(), err)
	}
	files = files[:0]
	return
}

func (s *Saver) openFileByType(typeOfSearch QueryCategory) (*os.File, error) {
	directory := filepath.Join(filepath.Dir(s.cfg.QueriesPath), string(typeOfSearch))
	err := os.MkdirAll(directory, os.ModePerm)
	if err != nil {
		return nil, err
	}
	newFilepath := filepath.Join(directory, queriesFileName)
	file, err := os.OpenFile(newFilepath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, os.ModePerm)
	if err != nil {
		return nil, err
	}
	_, err = file.Write([]byte(strings.Join([]string{"text", "query\n"}, s.cfg.CsvSeparator)))
	if err != nil {
		err = errors.Join(file.Close(), err)
		return nil, err
	}
	return file, nil
}

func NewSaver(cfg *config.Config, responseBodies chan []byte, done chan struct{}) (s *Saver, err error) {
	var file *os.File

	s = &Saver{
		responseBodies:       responseBodies,
		cfg:                  cfg,
		categoryToWriter:     make(map[QueryCategory]*bufio.Writer, len(allExistedValidTypes)),
		categoryToSetPresets: make(map[QueryCategory]map[string]struct{}),
		done:                 done,
	}
	s.files = make([]*os.File, 0, len(allExistedValidTypes))
	for _, queryType := range allExistedValidTypes {
		file, err = s.openFileByType(queryType)
		if err != nil {
			return nil, errors.Join(s.closeAll(), err)
		}
		s.files = append(s.files, file)
		s.categoryToWriter[queryType] = bufio.NewWriter(file)
		s.categoryToSetPresets[queryType] = make(map[string]struct{})
	}
	return s, nil
}

func (s *Saver) closeAll() error {
	return closeFiles(s.files)
}

func (s *Saver) flushAll() (err error) {
	for _, writer := range s.categoryToWriter {
		err = errors.Join(writer.Flush(), err)
	}
	return
}

func checkTokenInside(queryParams url.Values) (bool, error) {
	tokenInside := false
	for name, values := range queryParams {
		if len(values) == 0 {
			continue
		}
		matched, err := regexp.MatchString("_st[0-9]+", name)
		if err != nil {
			return false, err
		}
		if matched {
			tokenInside = true
			break
		}
	}
	return tokenInside, nil
}

func detectCategory(isTokenInside, isPresetInside bool) QueryCategory {
	if isPresetInside && isTokenInside {
		return extendSearch
	} else if isPresetInside && !isTokenInside {
		return preset
	} else if !isPresetInside && isTokenInside {
		return merger
	} else {
		return unknown
	}
}

func parseResponse(textBytes []byte) (queryInfo, error) {
	const semicolon = ";"
	respData := &ExactMatchResponse{}

	err := json.Unmarshal(textBytes, respData)
	if err != nil {
		return unknownQueryInfo, err
	}

	queryParamsStr := strings.Replace(respData.Metadata.CatalogValue, semicolon, url.PathEscape(semicolon), -1)
	queryParams, err := url.ParseQuery(queryParamsStr)
	if err != nil {
		return unknownQueryInfo, err
	}

	presetsStr, isPresetInside := queryParams["preset"]
	isTokenInside, err := checkTokenInside(queryParams)
	if err != nil {
		return unknownQueryInfo, err
	}

	qInfo := queryInfo{respData.Metadata.Name, unknown, respData.Metadata.CatalogValue, nil}
	qInfo.presets = presetsStr
	qInfo.category = detectCategory(isTokenInside, isPresetInside)

	return qInfo, nil
}

func (s *Saver) Run() (err error) {
	defer func() {
		if err != nil {
			close(s.done)
			for _ = range s.responseBodies {
				continue
			}
		}
		err = errors.Join(s.flushAll(), err)
		err = errors.Join(s.closeAll(), err)
		logger.GlobalLogger.Add(logger.Info, "Saver has finished")
	}()

	var qInfo queryInfo
	for value := range s.responseBodies {
		qInfo, err = parseResponse(value)
		if err != nil {
			return err
		}
		if qInfo.text == "" || qInfo.filterValue == "" {
			return errorCustom.InvalidEmptyResponse
		}
		if strings.Contains(qInfo.text, s.cfg.CsvSeparator) {
			return errorCustom.InvalidResponseSeparator(qInfo.text, s.cfg.CsvSeparator)
		}
		if strings.Contains(qInfo.filterValue, s.cfg.CsvSeparator) {
			return errorCustom.InvalidResponseSeparator(qInfo.filterValue, s.cfg.CsvSeparator)
		}
		text := strings.Join([]string{qInfo.text, qInfo.filterValue}, s.cfg.CsvSeparator) + "\n"
		_, err = s.categoryToWriter[qInfo.category].Write([]byte(text))
		if err != nil {
			return err
		}
		if presetMap, ok := s.categoryToSetPresets[qInfo.category]; ok {
			for _, presetId := range qInfo.presets {
				presetMap[presetId] = struct{}{}
			}
		}
		logger.GlobalLogger.Add(logger.Info, fmt.Sprintf("'%s' query has finished", qInfo.text))
	}
	err = s.writePresets()
	return err
}

func (s *Saver) getSortedPresets(setOfPresets map[string]struct{}) []string {
	presets := make([]string, 0, len(setOfPresets))
	for preset := range setOfPresets {
		presets = append(presets, preset)
	}
	slices.Sort(presets)
	return presets
}

func (s *Saver) writePresets() (err error) {
	var file *os.File
	for category, setOfPresets := range s.categoryToSetPresets {
		directory := filepath.Join(filepath.Dir(s.cfg.QueriesPath), string(category))
		pathToFile := filepath.Join(directory, presetsFileName)
		file, err = os.OpenFile(pathToFile, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, os.ModePerm)
		if err != nil {
			return err
		}
		s.files = append(s.files, file)
		writer := bufio.NewWriter(file)

		_, err = writer.Write(
			[]byte(strings.Join(s.getSortedPresets(setOfPresets), s.cfg.PresetsSeparator)),
		)
		if err != nil {
			return err
		}
		err = writer.Flush()
		if err != nil {
			return err
		}
	}
	return nil
}
