package pdf

import (
	"fmt"

	"github.com/bradhe/stopwatch"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func MergePdf(inFiles []string, outputFile string) {

	fmt.Printf("Fazendo merge ...")
	watch := stopwatch.Start()

	err := api.ValidateFiles(inFiles, nil)

	if err != nil {
		fmt.Println(err)
	}

	err = api.MergeAppendFile(inFiles, outputFile, nil)

	if err != nil {
		fmt.Println(err)
	}

	watch.Stop()
	fmt.Printf("concluído em: %v\n", watch.Milliseconds())
}
