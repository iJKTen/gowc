package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

type count struct {
  bytes int
  lines int
  chars int
  words int
  longest int
}

type flags struct {
  showBytes bool
  showLines bool
  showChars bool
  showWords bool
  showLongest bool
}

func manuallyParseArgs() (bool, []string, bool) {
	customFlagsSet := false
  hasPipedData := false
	var fileArgs []string
	var flagArgs []string

	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			customFlagsSet = true
			for _, char := range arg[1:] {
				flagArgs = append(flagArgs, "-"+string(char))
			}
		} else {
			fileArgs = append(fileArgs, arg)
		}
	}

	os.Args = append([]string{os.Args[0]}, append(flagArgs, fileArgs...)...)
	flag.Parse()

	meta, _ := os.Stdin.Stat()
  hasPipedData = meta.Mode() & os.ModeCharDevice == 0 

	if (hasPipedData && len(fileArgs) > 0) {
    hasPipedData = false
  }

	return customFlagsSet, fileArgs, hasPipedData
}

func resetFlags(flagArgs []string, linePtr, wordPtr, bytePtr, charsPtr *bool) {
	for _, arg := range flagArgs {
		if strings.HasPrefix(arg, "-") {
			switch arg[1:] {
			case "l":
				*linePtr = true
			case "w":
				*wordPtr = true
			case "c":
				*bytePtr = true
        *charsPtr = false
      case "m":
        *charsPtr = true
        *bytePtr = false
			}
		}
	}
}

func wc(reader io.Reader) *count {
  var (
    bytes int
    lines int
    words int
    chars int
    longest int
  )

  var wg sync.WaitGroup
  wg.Add(4)

  r1, w1 := io.Pipe()
  r2, w2 := io.Pipe()
  r3, w3 := io.Pipe()
  r4, w4 := io.Pipe()

  multiWrite := io.MultiWriter(w1, w2, w3, w4)

  go func() {
    defer w1.Close()
    defer w2.Close()
    defer w3.Close()
    defer w4.Close()
    io.Copy(multiWrite, reader)
  }()

  go func() {
    defer wg.Done()
    countWithSplitFunc(r1, bufio.ScanBytes, &bytes, nil)
  }()
  
  go func() {
    defer wg.Done()
    countWithSplitFunc(r2, bufio.ScanLines, &lines, &longest)
  }()

  go func() {
    defer wg.Done()
    countWithSplitFunc(r3, bufio.ScanWords, &words, nil)
  }()

  go func() {
    defer wg.Done()
    countRunesAndLonestLine(r4, &chars, &longest)
  }()

  wg.Wait()

  return &count{
    lines: lines,
    words: words,
    bytes: bytes,
    chars: chars,
    longest: longest,
  }
}

func countWithSplitFunc(reader io.Reader, splitfunc bufio.SplitFunc, result *int, result2 *int) {
  scanner := bufio.NewScanner(reader)
  scanner.Split(splitfunc)
  count := 0
  for scanner.Scan() {
    count++
    if result2 != nil {
      *result2 = max(*result2, len(scanner.Text()))
    }
  }

  if err := scanner.Err(); err != nil {
    fmt.Fprintf(os.Stderr, "Error during scan %v\n", err)
  }
  *result = count
}

func countRunesAndLonestLine(reader io.Reader, result *int, longest *int) {
  scanner := bufio.NewScanner(reader)
  scanner.Split(bufio.ScanRunes)

  currentLongestLine := 0
  for scanner.Scan() {
    rune := scanner.Text()
    *result++
    if rune == "\n" {
      if currentLongestLine > *longest {
        *longest = currentLongestLine
      }
      currentLongestLine = 0
    } else {
      currentLongestLine++
    }
  }

  if err := scanner.Err(); err != nil {
    fmt.Fprintf(os.Stderr, "Error during scan %v\n", err)
  }

  if currentLongestLine > *longest {
    *longest = currentLongestLine
  }
}

func display(counts *count, f *flags, fileName string) {
  if f.showLines {
    fmt.Printf("%8d", counts.lines)
  }

  if f.showWords {
    fmt.Printf("%8d", counts.words)
  }

  if f.showBytes {
    fmt.Printf("%8d", counts.bytes)
  } else if f.showChars {
    fmt.Printf("%8d", counts.chars)
  }

  if f.showLongest {
    fmt.Printf("%8d", counts.longest)
  }

  fmt.Printf(" %s\n", fileName)
}

func main() {
	linePtr := flag.Bool("l", true, "number of lines")
	wordPtr := flag.Bool("w", true, "number of words")
	bytePtr := flag.Bool("c", true, "number of bytes")
	charsPtr := flag.Bool("m", false, "number of characters")
	maxBytesLinePtr := flag.Bool("L", false, "length of line with most bytes")

	customFlagsSet, fileArgs, hasPipedData := manuallyParseArgs()

	if customFlagsSet {
		*linePtr = false
		*wordPtr = false
		*bytePtr = false

		resetFlags(os.Args[1:], linePtr, wordPtr, bytePtr, charsPtr)
	}

  aFlags := flags{
    showLines: *linePtr,
    showWords: *wordPtr,
    showBytes: *bytePtr,
    showChars: *charsPtr, 
    showLongest: *maxBytesLinePtr,
  } 

	if hasPipedData {
		reader := bufio.NewReader(os.Stdin)
    result := wc(reader)
    display(result, &aFlags, "")
    return
  }

  totals := count{}

	for _, fileName := range fileArgs {
    reader, err := os.Open(fileName)
    if err != nil {
      fmt.Fprintf(os.Stderr, "Error opening file %s", fileName)
      os.Exit(1)
    }

    result := wc(reader)
    display(result, &aFlags, fileName)
    totals.bytes = totals.bytes + result.bytes
    totals.lines = totals.lines + result.lines
    totals.words = totals.words + result.words
    totals.chars = totals.chars + result.chars
    totals.longest = max(totals.longest, result.longest)

    if err := reader.Close(); err != nil {
      fmt.Fprintf(os.Stderr, "Error during closing file %s: %v\n", fileName, err)
    }
	}

  if len(fileArgs) > 1 {
    display(&totals, &aFlags, "total")
  }
}
