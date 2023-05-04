package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dadosjusbr/status"
)

const (
	defaultGeneralTimeout  = 5 * time.Minute // Duração máxima total da coleta de todos os arquivos. Valor padrão calculado a partir de uma média de execuções ~4.5min
	defaulTimeBetweenSteps = 4 * time.Second //Tempo de espera entre passos do coletor."
)

func main() {
	if _, err := strconv.Atoi(os.Getenv("MONTH")); err != nil {
		status.ExitFromError(status.NewError(status.InvalidInput, fmt.Errorf("invalid month (\"%s\"): %w", os.Getenv("MONTH"), err)))
	}
	month := os.Getenv("MONTH")

	if _, err := strconv.Atoi(os.Getenv("YEAR")); err != nil {
		status.ExitFromError(status.NewError(status.InvalidInput, fmt.Errorf("invalid year (\"%s\"): %w", os.Getenv("YEAR"), err)))
	}
	year := os.Getenv("YEAR")

	outputFolder := os.Getenv("OUTPUT_FOLDER")
	if outputFolder == "" {
		outputFolder = "/output"
	}

	if err := os.Mkdir(outputFolder, os.ModePerm); err != nil && !os.IsExist(err) {
		status.ExitFromError(status.NewError(status.SystemError, fmt.Errorf("error creating output folder(%s): %w", outputFolder, err)))
	}

	generalTimeout := defaultGeneralTimeout
	if os.Getenv("GENERAL_TIMEOUT") != "" {
		var err error
		generalTimeout, err = time.ParseDuration(os.Getenv("GENERAL_TIMEOUT"))
		if err != nil {
			status.ExitFromError(status.NewError(status.InvalidInput, fmt.Errorf("invalid GENERAL_TIMEOUT (\"%s\"): %w", os.Getenv("GENERAL_TIMEOUT"), err)))
		}
	}

	timeBetweenSteps := defaulTimeBetweenSteps
	if os.Getenv("TIME_BETWEEN_STEPS") != "" {
		var err error
		timeBetweenSteps, err = time.ParseDuration(os.Getenv("TIME_BETWEEN_STEPS"))
		if err != nil {
			status.ExitFromError(status.NewError(status.InvalidInput, fmt.Errorf("invalid TIME_BETWEEN_STEPS (\"%s\"): %w", os.Getenv("TIME_BETWEEN_STEPS"), err)))
		}
	}
	c := crawler{
		collectionTimeout: generalTimeout,
		timeBetweenSteps:  timeBetweenSteps,
		year:              year,
		month:             month,
		output:            outputFolder,
	}
	downloads, err := c.crawl()
	if err != nil {
		status.ExitFromError(status.NewError(status.OutputError, err))
	}

	// O parser do MPAM espera os arquivos separados por \n. Mudanças aqui tem
	// refletir as expectativas lá.
	fmt.Println(strings.Join(downloads, "\n"))
}
