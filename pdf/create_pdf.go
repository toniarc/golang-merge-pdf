package pdf

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/bradhe/stopwatch"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

func create_pdf() {

	watch := stopwatch.Start()

	// create context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// capture pdf
	var buf []byte

	if err := chromedp.Run(ctx, printToPDF(`http://localhost:8161`, &buf)); err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("sample.pdf", buf, 0o644); err != nil {
		log.Fatal(err)
	}

	watch.Stop()
	fmt.Printf("Milliseconds elapsed: %v\n", watch.Milliseconds())
	fmt.Println("wrote sample.pdf")
}

// print a specific pdf page.
func printToPDF(urlstr string, res *[]byte) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Navigate(urlstr),
		chromedp.ActionFunc(func(ctx context.Context) error {
			buf, _, err := page.PrintToPDF().WithPrintBackground(true).Do(ctx)
			if err != nil {
				return err
			}
			*res = buf
			return nil
		}),
	}
}
