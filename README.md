# ArXiv Meta

Module for interfacing with the arxiv.org metadata API.

[ArXiv] provides a public API for accessing metadata of scientific papers.
 Documentation for the API can be found in the [ArXiv API User Manual].

 Basic usage:

```go
package main
import (
        "github.com/mikethicke/arxiv-go"
)
func main() {
        params := arxivmeta.SearchParams{
                Query: "all:electron",
        }
        requester := arxivmeta.MakeRequester(arxivgo.DefaultConfig)
        response, err := arxivmeta.Search(requester, params)
        if err != nil {
                panic(err)
        }
        for _, entry := range response.Entries {
             // Do something
        }
        nextPage, err := arxivmeta.SearchNext(requester, response)
        // Do something
}
```
 [ArXiv]: https:arxiv.org/
 [ArXiv API User Manual]: https:info.arxiv.org/help/api/user-manual.html