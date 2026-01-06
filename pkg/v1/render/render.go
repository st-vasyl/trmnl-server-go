package render

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
)

const (
	Width = 800
	Hight = 480
)

func RenderScreen() error {
	chromeUrl := "ws://192.168.64.4:9222"
	allocatorContext, _ := chromedp.NewRemoteAllocator(context.Background(), chromeUrl)

	// create a test server to serve the page
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Title</title>
</head>
<body>
<h1 id="title" class="link">
    <a href="https://test.com/helloworld">
        content of h1 1
    </a>
    <span>hello</span> world
</h1>
</body>
</html>
`,
		)
	}))
	defer ts.Close()

	var buf []byte

	d := 1 * time.Second
	var opts []chromedp.ContextOption
	ctx, _ := chromedp.NewContext(allocatorContext)
	opts = append(opts, chromedp.WithDebugf(log.Printf))
	if err := chromedp.Run(ctx,
		chromedp.EmulateViewport(Width, Hight),
		chromedp.Sleep(d),
		travelSubtree(ts.URL, `title`, chromedp.ByID),
		chromedp.CaptureScreenshot(&buf),
	); err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("public/elementScreenshot.png", buf, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("finished")
	return nil
}

func travelSubtree(urlstr string, sel interface{}, opts ...chromedp.QueryOption) chromedp.Tasks {
	// add populate option to the passed opts
	opts = append(opts, chromedp.Populate(-1, true, chromedp.PopulateWait(1*time.Second)))

	// retrieve the nodes
	var nodes []*cdp.Node
	return chromedp.Tasks{
		chromedp.Navigate(urlstr),
		// since the [chromedp.Populate] option has been added to opts, the
		// [chromedp.Nodes] action will wait until after the [chromedp.PopulateWait]
		// timeout has passed
		chromedp.Nodes(sel, &nodes, opts...),
		chromedp.ActionFunc(func(ctx context.Context) error {
			printNodes(os.Stdout, nodes, "", "  ")
			return nil
		}),
	}
}

func printNodes(w io.Writer, nodes []*cdp.Node, padding, indent string) {
	// This will block until the chromedp listener closes the channel
	for _, node := range nodes {
		switch {
		case node.NodeName == "#text":
			fmt.Fprintf(w, "%s#text: %q\n", padding, node.NodeValue)
		default:
			fmt.Fprintf(w, "%s%s:\n", padding, strings.ToLower(node.NodeName))
			if n := len(node.Attributes); n > 0 {
				fmt.Fprintf(w, "%sattributes:\n", padding+indent)
				for i := 0; i < n; i += 2 {
					fmt.Fprintf(w, "%s%s: %q\n", padding+indent+indent, node.Attributes[i], node.Attributes[i+1])
				}
			}
		}
		if node.ChildNodeCount > 0 {
			fmt.Fprintf(w, "%schildren:\n", padding+indent)
			printNodes(w, node.Children, padding+indent+indent, indent)
		}
	}
}
