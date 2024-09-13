package screenshots

import (
	"bufio"
	"errors"
	"fmt"
	"os"

	"wbx-script/queryVisualizer/config"
	"wbx-script/queryVisualizer/customerror"
	"wbx-script/searchType/logger"

	"github.com/playwright-community/playwright-go"
)

const screenshotFileSuffix = "_screenshot.jpg"

type ScreenshotMaker struct {
	cfg                *config.ScreenshotMakerConfig
	pagePool           chan playwright.Page
	playwrightInstance *playwright.Playwright
	browser            playwright.Browser
}

func NewScreenshotMaker(cfg *config.ScreenshotMakerConfig) (maker *ScreenshotMaker, rerr error) {
	defer func() {
		if rerr != nil && maker != nil {
			rerr = errors.Join(maker.Stop(), rerr)
		}
	}()

	var pw *playwright.Playwright

	if pw, rerr = playwright.Run(); rerr != nil {
		return nil, rerr
	}

	maker = &ScreenshotMaker{
		cfg:                cfg,
		playwrightInstance: pw,
		pagePool:           make(chan playwright.Page, cfg.PoolSize),
	}

	if err := maker.launchBrowser(); err != nil {
		return nil, err
	}

	for range cap(maker.pagePool) {
		page, err := maker.getPage()

		if err != nil {
			return nil, err
		}

		maker.pagePool <- page
	}

	return maker, nil
}

func (s *ScreenshotMaker) Stop() error {
	var err error

	for len(s.pagePool) > 0 {
		err = (<-s.pagePool).Close()
	}

	if s.browser != nil {
		err = errors.Join(s.browser.Close(), err)
	}

	if s.playwrightInstance != nil {
		err = errors.Join(s.playwrightInstance.Stop(), err)
	}

	return err
}

func (*ScreenshotMaker) writeScreenshot(prefixFilePath string, screenshot []byte) (err error) {
	var file *os.File

	if file, err = os.OpenFile(prefixFilePath+screenshotFileSuffix, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, os.ModePerm); err != nil {
		return err
	}

	defer func() {
		err = errors.Join(file.Close(), err)
	}()

	writer := bufio.NewWriter(file)
	defer func() {
		err = errors.Join(writer.Flush(), err)
	}()

	_, err = writer.Write(screenshot)

	return err
}

func (s *ScreenshotMaker) MakeScreenshot(presetsList, prefixFilePath string) (rerr error) {
	page := <-s.pagePool
	defer func() {
		if rerr != nil {
			rerr = customerror.ErrWrapStack("MakeScreenshot", rerr)
		}

		logger.Info(fmt.Sprintf("Screenshot file %q has been finished", prefixFilePath))
		s.pagePool <- page
	}()

	timeout := float64(s.cfg.Timeout.Milliseconds())

	if _, err := page.Goto(
		s.cfg.VisualizerURL.String(),
		playwright.PageGotoOptions{
			Timeout: &timeout,
		}); err != nil {
		return err
	}

	if err := page.Locator("textarea").First().Fill(presetsList, playwright.LocatorFillOptions{
		Timeout: &timeout,
	}); err != nil {
		return err
	}

	if err := page.Locator("button").First().Click(); err != nil {
		return err
	}

	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: &timeout,
	}); err != nil {
		return err
	}

	screenshot, err := page.Screenshot()

	if err != nil {
		return err
	}

	return s.writeScreenshot(prefixFilePath, screenshot)
}

func (s *ScreenshotMaker) getPage() (playwright.Page, error) {
	page, err := s.browser.NewPage()

	if err != nil {
		return nil, err
	}

	if err := page.SetViewportSize(s.cfg.Width, s.cfg.Height); err != nil {
		return nil, err
	}

	return page, nil
}

func (s *ScreenshotMaker) launchBrowser() error {
	if s.browser != nil {
		if errClose := s.browser.Close(); errClose != nil {
			return errClose
		}
	}

	var err error

	s.browser, err = s.playwrightInstance.Chromium.Launch()

	return err
}
