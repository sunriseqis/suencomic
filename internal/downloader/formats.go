package downloader

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jung-kurt/gofpdf"
	_ "golang.org/x/image/webp"
)

// prepareImageForPDF returns PDF-ready JPEG bytes plus the pixel dimensions.
// Images that are already JPEG are passed through untouched — re-encoding them
// would add a needless generation of quality loss and a full decode/encode
// cycle per page. Everything else is decoded and converted to JPEG.
func prepareImageForPDF(srcPath string) ([]byte, int, int, error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, 0, 0, err
	}

	if http.DetectContentType(data) == "image/jpeg" {
		if cfg, _, cfgErr := image.DecodeConfig(bytes.NewReader(data)); cfgErr == nil && cfg.Width > 0 {
			return data, cfg.Width, cfg.Height, nil
		}
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to decode image %s: %w", srcPath, err)
	}

	bounds := img.Bounds()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
		return nil, 0, 0, fmt.Errorf("failed to encode image as JPEG: %w", err)
	}

	return buf.Bytes(), bounds.Dx(), bounds.Dy(), nil
}

// MergeToPDF merges a list of local image files into a single PDF document
func MergeToPDF(outputPath string, imagePaths []string, title string) error {
	if len(imagePaths) == 0 {
		return fmt.Errorf("no images to merge into PDF")
	}

	_ = os.MkdirAll(filepath.Dir(outputPath), 0755)

	pdf := gofpdf.New("P", "pt", "A4", "")
	pdf.SetTitle(title, true)
	pdf.SetAuthor("Manga Downloader", true)

	for i, imgPath := range imagePaths {
		jpegData, width, height, err := prepareImageForPDF(imgPath)
		if err != nil {
			continue
		}

		imgName := fmt.Sprintf("page_%d", i+1)
		// Register image in memory
		opt := gofpdf.ImageOptions{
			ImageType: "JPEG",
			ReadDpi:   true,
		}
		pdf.RegisterImageOptionsReader(imgName, opt, bytes.NewReader(jpegData))

		// Add page matching the image aspect ratio
		// Convert pixels to points (1 pt = 1/72 inch, assume 96 dpi)
		ptW := float64(width) * 72.0 / 96.0
		ptH := float64(height) * 72.0 / 96.0
		if ptW <= 0 {
			ptW = 595.28
		}
		if ptH <= 0 {
			ptH = 841.89
		}

		pdf.AddPageFormat("P", gofpdf.SizeType{Wd: ptW, Ht: ptH})
		pdf.ImageOptions(imgName, 0, 0, ptW, ptH, false, opt, 0, "")
	}

	return pdf.OutputFileAndClose(outputPath)
}

// MergeToCBZ packages image files into a CBZ comic book archive
func MergeToCBZ(outputPath string, imagePaths []string, mangaTitle, chapterTitle string) error {
	if len(imagePaths) == 0 {
		return fmt.Errorf("no images to package into CBZ")
	}

	_ = os.MkdirAll(filepath.Dir(outputPath), 0755)
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	// Add ComicInfo.xml metadata
	comicInfoXML := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<ComicInfo xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema">
  <Series>%s</Series>
  <Number>%s</Number>
  <PageCount>%d</PageCount>
</ComicInfo>`, escapeXML(mangaTitle), escapeXML(chapterTitle), len(imagePaths))

	infoEntry, err := zipWriter.Create("ComicInfo.xml")
	if err == nil {
		_, _ = infoEntry.Write([]byte(comicInfoXML))
	}

	// Add images
	for i, imgPath := range imagePaths {
		ext := filepath.Ext(imgPath)
		if ext == "" {
			ext = ".jpg"
		}
		entryName := fmt.Sprintf("%04d%s", i+1, ext)

		fileData, rErr := os.ReadFile(imgPath)
		if rErr != nil {
			continue
		}

		w, cErr := zipWriter.Create(entryName)
		if cErr != nil {
			continue
		}
		_, _ = w.Write(fileData)
	}

	return nil
}

// MergeToEPUB packages images into a standard EPUB comic ebook
func MergeToEPUB(outputPath string, imagePaths []string, mangaTitle, chapterTitle string) error {
	if len(imagePaths) == 0 {
		return fmt.Errorf("no images to package into EPUB")
	}

	_ = os.MkdirAll(filepath.Dir(outputPath), 0755)
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	// 1. mimetype (must be first, uncompressed)
	mimeHeader := &zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store,
	}
	mimeWriter, err := zipWriter.CreateHeader(mimeHeader)
	if err != nil {
		return err
	}
	_, _ = mimeWriter.Write([]byte("application/epub+zip"))

	// 2. META-INF/container.xml
	containerXML := `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
    <rootfiles>
        <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
    </rootfiles>
</container>`
	cWriter, _ := zipWriter.Create("META-INF/container.xml")
	if cWriter != nil {
		_, _ = cWriter.Write([]byte(containerXML))
	}

	// 3. Process Images & Generate XHTML pages
	type pageEntry struct {
		ImageFilename string
		PageFilename  string
		ImageID       string
		PageID        string
		MimeType      string
	}
	var pages []pageEntry

	for i, imgPath := range imagePaths {
		ext := strings.ToLower(filepath.Ext(imgPath))
		mime := "image/jpeg"
		if ext == ".png" {
			mime = "image/png"
		} else if ext == ".webp" {
			mime = "image/webp"
		} else if ext == ".gif" {
			mime = "image/gif"
		} else {
			ext = ".jpg"
		}

		imgFileName := fmt.Sprintf("image_%04d%s", i+1, ext)
		pageFileName := fmt.Sprintf("page_%04d.xhtml", i+1)

		p := pageEntry{
			ImageFilename: imgFileName,
			PageFilename:  pageFileName,
			ImageID:       fmt.Sprintf("img_%d", i+1),
			PageID:        fmt.Sprintf("page_%d", i+1),
			MimeType:      mime,
		}
		pages = append(pages, p)

		// Write image file into OEBPS/images/
		data, rErr := os.ReadFile(imgPath)
		if rErr == nil {
			w, _ := zipWriter.Create("OEBPS/images/" + imgFileName)
			if w != nil {
				_, _ = w.Write(data)
			}
		}

		// Write XHTML page
		pageContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml">
<head>
    <title>Page %d</title>
    <style>
        body { margin: 0; padding: 0; text-align: center; background-color: #000; }
        img { max-width: 100%%; max-height: 100vh; height: auto; display: block; margin: 0 auto; }
    </style>
</head>
<body>
    <img src="images/%s" alt="Page %d" />
</body>
</html>`, i+1, imgFileName, i+1)

		pw, _ := zipWriter.Create("OEBPS/" + pageFileName)
		if pw != nil {
			_, _ = pw.Write([]byte(pageContent))
		}
	}

	// 4. OEBPS/content.opf
	var manifestItems strings.Builder
	var spineItems strings.Builder

	manifestItems.WriteString(`        <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>` + "\n")
	for _, p := range pages {
		manifestItems.WriteString(fmt.Sprintf(`        <item id="%s" href="images/%s" media-type="%s"/>`+"\n", p.ImageID, p.ImageFilename, p.MimeType))
		manifestItems.WriteString(fmt.Sprintf(`        <item id="%s" href="%s" media-type="application/xhtml+xml"/>`+"\n", p.PageID, p.PageFilename))
		spineItems.WriteString(fmt.Sprintf(`        <itemref idref="%s"/>`+"\n", p.PageID))
	}

	contentOPF := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" unique-identifier="BookID" version="2.0">
    <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
        <dc:title>%s - %s</dc:title>
        <dc:language>zh</dc:language>
        <dc:creator>Manga Downloader</dc:creator>
        <dc:identifier id="BookID">urn:uuid:%s-%s</dc:identifier>
    </metadata>
    <manifest>
%s
    </manifest>
    <spine toc="ncx">
%s
    </spine>
</package>`, escapeXML(mangaTitle), escapeXML(chapterTitle), escapeXML(mangaTitle), escapeXML(chapterTitle), manifestItems.String(), spineItems.String())

	opfw, _ := zipWriter.Create("OEBPS/content.opf")
	if opfw != nil {
		_, _ = opfw.Write([]byte(contentOPF))
	}

	// 5. OEBPS/toc.ncx
	tocNCX := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE ncx PUBLIC "-//NISO//DTD ncx 2005-1//EN" "http://www.daisy.org/z3986/2005/ncx-2005-1.dtd">
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
    <head>
        <meta name="dtb:uid" content="urn:uuid:%s-%s"/>
        <meta name="dtb:depth" content="1"/>
        <meta name="dtb:totalPageCount" content="%d"/>
        <meta name="dtb:maxPageNumber" content="%d"/>
    </head>
    <docTitle><text>%s - %s</text></docTitle>
    <navMap>
        <navPoint id="navpoint-1" playOrder="1">
            <navLabel><text>%s</text></navLabel>
            <content src="page_0001.xhtml"/>
        </navPoint>
    </navMap>
</ncx>`, escapeXML(mangaTitle), escapeXML(chapterTitle), len(pages), len(pages), escapeXML(mangaTitle), escapeXML(chapterTitle), escapeXML(chapterTitle))

	ncxw, _ := zipWriter.Create("OEBPS/toc.ncx")
	if ncxw != nil {
		_, _ = ncxw.Write([]byte(tocNCX))
	}

	return nil
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
