package saver

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"wbx-script/searchType/config"
	"wbx-script/searchType/logger"
)

type QueryCategory string

const (
	preset       QueryCategory = "Preset"
	extendSearch QueryCategory = "ExtendSearch"
	merger       QueryCategory = "Merger"
	unknown      QueryCategory = "Unknown"
)

const (
	presetsFileName = "list.presets"
	queriesFileName = "queries.csv"
)

const (
	semicolon = ";"
	newline   = "\n"
)

var allExistedValidTypes = []QueryCategory{preset, extendSearch, merger}

type queryInfo struct {
	text        string
	category    QueryCategory
	filterValue string
	presets     []string
}

type CategoryPresetsMap map[QueryCategory]map[string]struct{}

var unknownQueryInfo = queryInfo{"", unknown, "", nil}

type ExactMatchResponse struct {
	Metadata struct {
		Name         string `json:"name"`
		CatalogValue string `json:"catalog_value"`
	} `json:"metadata"`
}

type Saver struct {
	cfg                  *config.Config
	responseBodies       <-chan []byte
	categoryToWriter     map[QueryCategory]*bufio.Writer
	categoryToSetPresets CategoryPresetsMap
	openedFiles          []*os.File
}

func closeFiles(files []*os.File) error {
	var err error

	for i := range files {
		err = errors.Join(files[i].Close(), err)
	}

	return err
}

func (s *Saver) openFileByType(typeOfSearch QueryCategory) (*os.File, error) {
	directory := filepath.Join(filepath.Dir(s.cfg.QueriesPath), string(typeOfSearch))

	if err := os.MkdirAll(directory, os.ModePerm); err != nil {
		return nil, err
	}

	newFilepath := filepath.Join(directory, queriesFileName)
	file, err := os.OpenFile(newFilepath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, os.ModePerm)

	if err != nil {
		return nil, err
	}

	if _, err = file.WriteString("text" + s.cfg.CsvSeparator + "query" + newline); err != nil {
		return nil, errors.Join(file.Close(), err)
	}

	return file, nil
}

func NewSaver(cfg *config.Config, responseBodies <-chan []byte) (*Saver, error) {
	var (
		err  error
		file *os.File
	)

	s := &Saver{
		responseBodies:       responseBodies,
		cfg:                  cfg,
		categoryToWriter:     make(map[QueryCategory]*bufio.Writer, len(allExistedValidTypes)),
		categoryToSetPresets: make(CategoryPresetsMap),
	}
	s.openedFiles = make([]*os.File, 0, len(allExistedValidTypes))

	for _, queryType := range allExistedValidTypes {
		file, err = s.openFileByType(queryType)

		if err != nil {
			return nil, errors.Join(s.closeAll(), err)
		}

		s.openedFiles = append(s.openedFiles, file)
		s.categoryToWriter[queryType] = bufio.NewWriter(file)
		s.categoryToSetPresets[queryType] = make(map[string]struct{})
	}

	return s, nil
}

func (s *Saver) closeAll() error {
	if err := closeFiles(s.openedFiles); err != nil {
		return err
	}

	s.openedFiles = s.openedFiles[:0]

	return nil
}

func (s *Saver) flushAll() error {
	var err error

	for _, writer := range s.categoryToWriter {
		err = errors.Join(writer.Flush(), err)
	}

	return err
}

func checkTokenInside(queryParams url.Values) bool {
	regex := regexp.MustCompile(`_st\d+`)

	for name, values := range queryParams {
		if len(values) == 0 {
			continue
		}

		if matched := regex.MatchString(name); matched {
			return true
		}
	}

	return false
}

func detectCategory(isTokenInside, isPresetInside bool) QueryCategory {
	if isPresetInside && isTokenInside {
		return extendSearch
	}

	if isPresetInside {
		return preset
	}

	if isTokenInside {
		return merger
	}

	return unknown
}

func parseResponse(textBytes []byte) (queryInfo, error) {
	respData := &ExactMatchResponse{}

	if err := json.Unmarshal(textBytes, respData); err != nil {
		return unknownQueryInfo, err
	}

	queryParamsStr := strings.ReplaceAll(respData.Metadata.CatalogValue, semicolon, url.PathEscape(semicolon))
	queryParams, err := url.ParseQuery(queryParamsStr)

	if err != nil {
		return unknownQueryInfo, err
	}

	presetsStr, isPresetInside := queryParams["preset"]
	isTokenInside := checkTokenInside(queryParams)

	qInfo := queryInfo{respData.Metadata.Name, unknown, respData.Metadata.CatalogValue, nil}
	qInfo.presets = presetsStr
	qInfo.category = detectCategory(isTokenInside, isPresetInside)

	return qInfo, nil
}

func (s *Saver) validate(info queryInfo) error {
	if info.text == "" || info.filterValue == "" {
		return ErrEmptyResponse
	}

	if strings.Contains(info.text, s.cfg.CsvSeparator) {
		return ErrInvalidResponseSeparator(info.text, s.cfg.CsvSeparator)
	}

	if strings.Contains(info.filterValue, s.cfg.CsvSeparator) {
		return ErrInvalidResponseSeparator(info.filterValue, s.cfg.CsvSeparator)
	}

	if info.category == unknown {
		return ErrUnknownCategory
	}

	return nil
}

func (s *Saver) Run(_ context.Context) (err error) {
	defer func() {
		err = errors.Join(s.flushAll(), s.closeAll(), err)

		logger.Info("Saver has finished")
	}()

	var qInfo queryInfo

	for value := range s.responseBodies {
		qInfo, err = parseResponse(value)

		if err != nil {
			return err
		}

		if err = s.validate(qInfo); err != nil {
			switch {
			case errors.Is(err, ErrUnknownCategory):
				logger.Info(fmt.Sprintf("Query %q has been skipped", qInfo.text))
				continue
			default:
				return err
			}
		}

		text := qInfo.text + s.cfg.CsvSeparator + qInfo.filterValue + newline

		if _, err = s.categoryToWriter[qInfo.category].WriteString(text); err != nil {
			return err
		}

		if presetMap, ok := s.categoryToSetPresets[qInfo.category]; ok {
			for _, presetID := range qInfo.presets {
				presetMap[presetID] = struct{}{}
			}
		}

		logger.Info(fmt.Sprintf("%q query has finished", qInfo.text))
	}

	return s.writePresets()
}

func (*Saver) getSortedPresets(setOfPresets map[string]struct{}) []string {
	presets := make([]string, 0, len(setOfPresets))
	for preset := range setOfPresets {
		presets = append(presets, preset)
	}

	slices.Sort(presets)

	return presets
}

func (s *Saver) writePresets() error {
	var errSum error

	for category, setOfPresets := range s.categoryToSetPresets {
		directory := filepath.Join(filepath.Dir(s.cfg.QueriesPath), string(category))
		pathToFile := filepath.Join(directory, presetsFileName)
		file, err1 := os.OpenFile(pathToFile, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, os.ModePerm)

		if err1 != nil {
			errSum = errors.Join(err1, errSum)
			continue
		}

		s.openedFiles = append(s.openedFiles, file)

		writer := bufio.NewWriter(file)

		if _, err := writer.WriteString(
			strings.Join(s.getSortedPresets(setOfPresets), s.cfg.PresetsSeparator),
		); err != nil {
			errSum = errors.Join(err, errSum)
			continue
		}

		if err := writer.Flush(); err != nil {
			errSum = errors.Join(err, errSum)
			continue
		}
	}

	return errSum
}
