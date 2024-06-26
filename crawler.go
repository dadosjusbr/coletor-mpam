package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
	"github.com/dadosjusbr/status"
)

type crawler struct {
	collectionTimeout time.Duration
	timeBetweenSteps  time.Duration
	year              string
	month             string
	output            string
}

func (c crawler) crawl() ([]string, error) {
	// Chromedp setup.
	log.SetOutput(os.Stderr) // Enviando logs para o stderr para não afetar a execução do coletor.
	alloc, allocCancel := chromedp.NewExecAllocator(
		context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true), // mude para false para executar com navegador visível.
			chromedp.Flag("ignore-certificate-errors", "1"),
			chromedp.NoSandbox,
			chromedp.DisableGPU,
		)...,
	)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(
		alloc,
		chromedp.WithLogf(log.Printf), // remover comentário para depurar
	)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, c.collectionTimeout)
	defer cancel()

	// NOTA IMPORTANTE: os prefixos dos nomes dos arquivos tem que ser igual
	// ao esperado no parser MPAM.

	// Contracheque
	log.Printf("Realizando seleção (%s/%s)...", c.month, c.year)
	if err := c.abreCaixaDialogo(ctx, "contracheque"); err != nil {
		status.ExitFromError(err)
	}
	log.Printf("Seleção realizada com sucesso!\n")
	cqFname := c.downloadFilePath("contracheque")
	log.Printf("Fazendo download do contracheque (%s)...", cqFname)
	if err := c.exportaPlanilha(ctx, cqFname); err != nil {
		status.ExitFromError(err)
	}
	log.Printf("Download realizado com sucesso!\n")

	// Indenizações
	log.Printf("Realizando seleção (%s/%s)...", c.month, c.year)
	if err := c.abreCaixaDialogo(ctx, "indenizatorias"); err != nil {
		status.ExitFromError(err)
	}
	log.Printf("Seleção realizada com sucesso!\n")
	iFname := c.downloadFilePath("indenizatorias")
	log.Printf("Fazendo download das indenizações (%s)...", iFname)
	if err := c.exportaPlanilha(ctx, iFname); err != nil {
		status.ExitFromError(err)
	}
	log.Printf("Download realizado com sucesso!\n")

	// Retorna caminhos completos dos arquivos baixados.
	return []string{cqFname, iFname}, nil
}

func (c crawler) downloadFilePath(prefix string) string {
	return filepath.Join(c.output, fmt.Sprintf("membros-ativos-%s-%s-%s.xls", prefix, c.month, c.year))
}

func (c crawler) abreCaixaDialogo(ctx context.Context, tipo string) error {
	var baseURL string
	selectYear := `//*[@id="SC_data"]`
	if tipo == "contracheque" {
		baseURL = "https://contrachequetransparencia.mpam.mp.br/grid_VW_TRANSPARENCIA_GERAL/"

		if err := chromedp.Run(ctx,
			chromedp.Navigate(baseURL),
			chromedp.Sleep(c.timeBetweenSteps),

			// Seleciona ano
			chromedp.SetValue(selectYear, fmt.Sprintf("%s/%s##@@%s/%s", c.month, c.year, c.month, c.year), chromedp.BySearch),
			chromedp.Sleep(c.timeBetweenSteps),

			// Seleciona mes
			chromedp.SetValue(`//*[@id="SC_classificacao"]`, "MEMBROS ATIVOS##@@MEMBROS ATIVOS", chromedp.BySearch, chromedp.NodeVisible),
			chromedp.Sleep(c.timeBetweenSteps),

			// Busca
			chromedp.Click(`//*[@id="sc_b_pesq_bot"]`, chromedp.BySearch, chromedp.NodeVisible),
			chromedp.Sleep(c.timeBetweenSteps),

			// Altera o diretório de download
			browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllowAndName).
				WithDownloadPath(c.output).
				WithEventsEnabled(true),
		); err != nil {
			// Caso haja erro na coleta, verificamos se este erro é por não haver dados e retornamos status 4.
			if strings.Contains(err.Error(), "could not set value on node") {
				return status.NewError(status.DataUnavailable, fmt.Errorf("não há dados disponíveis de contracheques para %s/%s: %w", c.month, c.year, err))
			} else {
				return status.NewError(status.ConnectionError, fmt.Errorf("erro abrindo caixa da planilha de contracheque: %w", err))
			}
		}
	} else {
		baseURL = "https://contrachequetransparencia.mpam.mp.br/grid_TRANSPARENCIA_INDENIZACAO/"

		if err := chromedp.Run(ctx,
			chromedp.Navigate(baseURL),
			chromedp.Sleep(c.timeBetweenSteps),

			// Seleciona ano
			chromedp.SetValue(selectYear, fmt.Sprintf("%s/%s##@@%s/%s", c.month, c.year, c.month, c.year), chromedp.BySearch),
			chromedp.Sleep(c.timeBetweenSteps),

			// Busca
			chromedp.Click(`//*[@id="sc_b_pesq_bot"]`, chromedp.BySearch, chromedp.NodeVisible),
			chromedp.Sleep(c.timeBetweenSteps),

			// Altera o diretório de download
			browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllowAndName).
				WithDownloadPath(c.output).
				WithEventsEnabled(true),
		); err != nil {
			// Caso haja erro na coleta, verificamos se este erro é por não haver dados e retornamos status 4.
			if strings.Contains(err.Error(), "could not set value on node") {
				return status.NewError(status.DataUnavailable, fmt.Errorf("não há dados disponíveis de indenizações para %s/%s: %w", c.month, c.year, err))
			} else {
				return status.NewError(status.ConnectionError, fmt.Errorf("erro abrindo caixa da planilha de verbas indenizatorias: %w", err))
			}
		}
	}
	return nil
}

// exportaPlanilha clica no botão correto para exportar para excel, espera um tempo para download renomeia o arquivo.
func (c crawler) exportaPlanilha(ctx context.Context, fName string) error {
	tctx, tcancel := context.WithTimeout(ctx, 30*time.Second)
	defer tcancel()
	if err := chromedp.Run(tctx,
		// Clica no botão de download
		chromedp.Click(`//*[@id="sc_btgp_btn_group_1_top"]`, chromedp.BySearch, chromedp.NodeVisible),
		chromedp.Sleep(c.timeBetweenSteps),
	); err != nil {
		return status.NewError(status.DataUnavailable, fmt.Errorf("não há dados disponíveis"))
	}
	if err := chromedp.Run(ctx,
		chromedp.Click(`//*[@id="xls_top"]`, chromedp.BySearch, chromedp.NodeVisible),
		chromedp.Sleep(c.timeBetweenSteps),

		chromedp.Click(`//*[@id="idBtnDown"]`, chromedp.BySearch, chromedp.NodeVisible),
		chromedp.Sleep(c.timeBetweenSteps),
	); err != nil {
		return status.NewError(status.ConnectionError, fmt.Errorf("falha no download: %w", err))
	}

	if err := nomeiaDownload(c.output, fName); err != nil {
		return status.NewError(status.SystemError, fmt.Errorf("erro renomeando arquivo (%s): %w", fName, err))
	}
	if _, err := os.Stat(fName); os.IsNotExist(err) {
		return status.NewError(status.SystemError, fmt.Errorf("download do arquivo de %s não realizado: %w", fName, err))
	}
	return nil
}

// nomeiaDownload dá um nome ao último arquivo modificado dentro do diretório
// passado como parâmetro nomeiaDownload dá pega um arquivo
func nomeiaDownload(output, fName string) error {
	// Identifica qual foi o ultimo arquivo
	files, err := os.ReadDir(output)
	if err != nil {
		return status.NewError(status.SystemError, fmt.Errorf("erro lendo diretório %s: %w", output, err))
	}
	var newestFPath string
	var newestTime int64 = 0
	for _, f := range files {
		fPath := filepath.Join(output, f.Name())
		fi, err := os.Stat(fPath)
		if err != nil {
			return status.NewError(status.SystemError, fmt.Errorf("erro obtendo informações sobre arquivo %s: %w", fPath, err))
		}
		currTime := fi.ModTime().Unix()
		if currTime > newestTime {
			newestTime = currTime
			newestFPath = fPath
		}
	}
	// Renomeia o ultimo arquivo modificado.
	if err := os.Rename(newestFPath, fName); err != nil {
		return status.NewError(status.SystemError, fmt.Errorf("erro renomeando último arquivo modificado (%s)->(%s): %w", newestFPath, fName, err))
	}
	return nil
}
