package main

import (
    "fmt"
    "io/ioutil"
    "os"
    "log"
    "bytes"
    "strings"
    "html"
    "flag"
    "os/user"
    "path"
    "path/filepath"
    "html/template"
    "sort"
    "time"
    "crypto/md5"
    "encoding/hex"

    "github.com/gobuffalo/packr"
    "github.com/json-iterator/go"
)

type pigconf struct {
    style string
    // private variables
    imgin_ string  // Location where read images from
    imgout_ string // Where images located in when output
}
var pc pigconf

type postmeta struct {
    Title string
    Date string
    Author string
    Link template.URL // disable backslash escaping
    Latest string // the latest update date
}

func getHeadline(block []byte) (map[string]string) {
    headline := make(map[string]string)
    lines := bytes.Split(block, []byte{0xa})
    if string(lines[0]) != "---" || string(lines[len(lines) - 1]) != "---" {
        log.Fatal("Wrong meta format!\n")
    }
    // Remove the first and last "---" from headline,
    // you know that the slice in go is really silly, it should not support negative index!
    lines = lines[1:len(lines) - 1]
    for _, line := range lines {
        s := string(line)
        info := strings.Split(s, ":")
        if len(info) < 2 {
            log.Fatal("The format of <", s , "> is not correct!\n")
        }
        key := strings.ToLower(strings.TrimSpace(info[0]))
        val := strings.TrimSpace(info[1])
        headline[key] = val
    }
    return headline
}

func renderLine(block []byte) (string){
    htmlline := ""
    line := []rune(string(block))
    // for rune, len returns the number of character
    // i is the index of unicode character
    for i := 0; i < len(line); i++ {
        switch line[i] {
            case '`' :
                remain := string(line[i + 1:])
                // well, the fuck! idx is the byte index but not unicode!
                idx := strings.IndexRune(remain, '`')
                if idx != -1 {
                    blk := string(remain[0: idx])
                    htmlline += `&nbsp;<code class="language-clike">` + html.EscapeString(blk) + "</code>&nbsp;"
                    // notice that we should calculate the number of unicode and accumulate
                    i += 1 + len([]rune(blk))
                } else {
                    htmlline += html.EscapeString(string(line[i]))
                }
            case '@':
                if i + 1 < len(line) && line[i + 1] == '[' {
                    remain := string(line[i+1:])
                    idx := strings.IndexRune(remain, ']')
                    // find ']' and there is at least one character in '[]'
                    if idx != -1 && idx > 1{
                        blk := strings.TrimSpace(string(remain[1:idx]))
                        // lambda function to check if the link is an image
                        isimg := func(link string) bool {
                            if dotidx := strings.LastIndex(link, "."); dotidx != -1 {
                                link = link[dotidx:]
                                switch link {
                                case ".gif", ".png", ".jpg", ".jpeg", ".svg":
                                    return true
                                default:
                                    return false
                                }
                            } else {
                                return false
                            }
                        }(blk)
                        if isimg {
                            // copy image to destination dir
                            inimg := expandPath(filepath.Join(pc.imgin_, blk))
                            if _, err := os.Stat(pc.imgout_); os.IsNotExist(err) {
                                os.Mkdir(pc.imgout_, os.ModePerm)
                            }
                            outimg := filepath.Join(pc.imgout_, path.Base(blk))
                            // fmt.Printf("inimg: %s outimg: %s\n", inimg, outimg)
                            // avoid copy same image to itself
                            if inimg != outimg {
                                imgdata, _ := ioutil.ReadFile(inimg)
                                ioutil.WriteFile(outimg, imgdata, os.ModePerm)
                            }
                            htmlline += fmt.Sprintf("<img src=\"images/%s\"/>", path.Base(blk))
                        } else {
                            link := blk
                            if len(blk) > 32 {
                                link = blk[0:32] + "..."
                            }
                            htmlline += fmt.Sprintf("<a href=\"%s\">%s</a>", blk, link)
                        }
                        i += 2 + len(blk)
                    } else {
                        htmlline += html.EscapeString(string(line[i]))
                    }
                } else {
                    htmlline += html.EscapeString(string(line[i]))
                }
            default:
                htmlline += html.EscapeString(string(line[i]))
        }
    }
    return htmlline
}

func renderPara(block []byte) (string) {
    lines := bytes.Split(block, []byte{0xa})
    para := "<p>"
    for _, line := range lines {
        para += renderLine(line)
    }
    return para + "</p>"
}

type Stack struct {
    data []string;
    l int;
}

func NewStack() *Stack {
    stack := new(Stack)
    stack.data = make([]string, 0)
    stack.l = 0
    return stack
}

func (s *Stack) Push(item string) {
    s.data = append(s.data, item)
    s.l += 1
}

func (s *Stack) Pop() (string) {
    if s.l > 0 {
        item := s.data[s.l - 1]
        s.data = s.data[0:s.l - 1]
        s.l -= 1
        return item
    } else {
        return ""
    }
}

func (s *Stack) Size() (int) {
    return s.l;
}

func (s *Stack) Print() {
    for idx, item := range s.data {
        fmt.Printf("[%d] = %s\n", idx, item)
    }
}

func renderList(btlines [][]byte) (string) {
    stack := NewStack()
    listhtml := "<ul>"
    stack.Push("</ul>")
    // indent level of the current list item (based 0)
    level := 0
    firstitem := true
    for i := 0; i < len(btlines); i++ {
        line := strings.TrimRight(string(btlines[i]), " ")
        // If there should an item, then string '-' must be the first non-blank character
        space := len(line) - len(strings.TrimLeft(line, " "))
        // The index of item indicator '- '
        idx := strings.Index(line, "- ")
        // if the idx is not found or the item indicator is not the first
        if idx == -1 || line[space:space + 2] != "- " {
            // the least indent value for list-nested codes
            codeindent := (level + 2) * 4
            if space >= codeindent {
                codeblk := make([]byte, 0, 64)
                for j := i; j < len(btlines); j++ {
                    spacenum := len(btlines[j]) - len(bytes.TrimLeft(btlines[j], " "))
                    // gather code lines
                    if spacenum >= codeindent {
                        codeblk = append(codeblk, btlines[j]...)
                        codeblk = append(codeblk, 0xa)
                    }
                    // render codes
                    if spacenum < codeindent || j == len(btlines) - 1 {
                        tmp := renderCode(codeblk, codeindent)
                        listhtml += tmp
                        if j == len(btlines) - 1 {
                            i = j
                        } else {
                            i = j - 1
                        }
                        break
                    }
                }
            } else {
                listhtml += renderLine(([]byte(line)))
            }
        } else {
            brk := ""
            if line[len(line) - 1] == ':' {
                line = line[0:len(line) - 1]
                brk = "<br/>"
            }
            // idx will be 4 * i for any i >= 0 (idx is counted from 0)
            if idx / 4 == level {
                // Case 1: Keep going on the current level
                if firstitem {
                    firstitem = false;
                } else {
                    // If the item in the same level and the item is not the global first one,
                    // then there should be an end mark
                    listhtml += stack.Pop()
                    firstitem = false
                }
                listhtml += "<li>"
                listhtml += renderLine([]byte(line[idx + 2:])) + brk
                stack.Push("</li>")
            } else if idx / 4 > level {
                // Case 2: Nested level
                listhtml += "<ul>"
                stack.Push("</ul>")
                listhtml += "<li>"
                listhtml += renderLine([]byte(line[idx + 2:])) + brk
                stack.Push("</li>")
                level = idx / 4
            } else {
                // Case 3: Go back up level
                for j := idx / 4; j < level; j++ {
                    listhtml += stack.Pop() // pop </li>
                    listhtml += stack.Pop() // pop </ul>
                }
                listhtml += stack.Pop()  // pop </li>
                listhtml += "<li>"
                listhtml += renderLine([]byte(line[idx + 2:])) + brk
                stack.Push("</li>")
                level = idx / 4
            }
        }
    }
    for stack.Size() > 0 {
        listhtml += stack.Pop()
    }
    return listhtml
}

func renderTitle(block []byte) string {
    line := strings.TrimSpace(string(block))
    // I do not know why strings.TrimPrefix does not work
    // title := strings.TrimPrefix(line, "#")
    title := strings.TrimLeft(line, "#")
    level := len(line) - len(title)

    // check corner case: '###' => no title
    if len(title) == 0 {
        title = "[NO TITLE]"
    }

    // The w3c specification says that The heading elements are H1, H2, H3, H4, H5, and H6
    // with H1 being the highest (or most important) level and H6 the least.
    // As result, if the title level is more than 6, then it shuold be six
    if level > 6 {
        fmt.Printf("[Warn] {%s} => Too many title levels, and will be reset to <h6></h6>!\n", line)
        level = 6
    }
    return fmt.Sprintf("<h%d>%s</h%d>", level, title, level)
}

func renderCode(block []byte, outindent int) string {
    btlines := bytes.Split(block, []byte{0xa})
    idx := strings.Index(string(btlines[0]), "//:")
    highlights := "language-clike"
    if idx != -1 {
        highlights = "language-" + strings.TrimSpace(string(btlines[0])[idx + 3:])
    }
    code := fmt.Sprintf("<pre><code class=\"%s\">", highlights)
    for no, btline := range btlines {
        // skip highlight line
        if idx != -1 && no == 0 {
            continue
        }
        // if the last line is empty, we skip it
        if no == len(btlines) - 1 && len(bytes.TrimSpace(btline)) == 0 {
            continue
        }
        space := len(btline) - len(bytes.TrimSpace(btline))
        if outindent > space {
            outindent = 0
        }
        line := html.EscapeString(string(btline[outindent:]))
        code += line + "\n"
    }
    code += "</code></pre>"
    return code
}

func getBlockType(block []byte) string {
    block = bytes.TrimPrefix(block, []byte{0xa})
    lines := bytes.Split(block, []byte{0xa})
    flag := string(lines[0])
    if len(flag) >= 1 && flag[0] == '#' {
        return "TITLE"
    } else if len(flag) >= 3 && flag == "---" {
        return "META"
    } else if len(flag) >= 2 && flag[0:2] == "- " {
        return "LIST"
    } else if len(flag) >= 4 && flag[0:4] == "    " {
        if len(flag) >= 8 && flag[0:8] == "        " {
            return "CODE8"
        } else {
            return "CODE4"
        }
    } else {
        return "PARA"
    }
}

func splitFile(infile string) [][]byte {
    input, err := ioutil.ReadFile(infile)
    if err != nil {
        log.Fatal("Cannot read input file!")
    }
    chunks := make([][]byte, 0)
    blocks := bytes.Split(input[0:], []byte{0xa, 0xa})
    for i := 0; i < len(blocks); i++ {
        chunk := make([]byte, 0)
        // merge CODE4 block
        if getBlockType(blocks[i]) == "CODE4" {
            j := i
            for ; j < len(blocks) && strings.HasPrefix(getBlockType(blocks[j]), "CODE"); j++ {
                chunk = append(chunk, 0xa)
                chunk = append(chunk, blocks[j]...)
                // well, it is really tricky
                chunk = append(chunk, "\n    "...)
            }
            i = j - 1
        // merge CODE8 block
        } else if getBlockType(blocks[i]) == "LIST" {
            chunk = append(chunk, blocks[i]...)
            j := i + 1
            for ; j < len(blocks) && strings.HasPrefix(getBlockType(blocks[j]), "CODE"); j++ {
                chunk = append(chunk, 0xa)
                chunk = append(chunk, "        \n"...)
                chunk = append(chunk, blocks[j]...)
            }
            i = j - 1
        } else {
            chunk = append(chunk, 0xa)
            chunk = append(chunk, blocks[i]...)
        }
        chunks = append(chunks, chunk)
    }
    return chunks
}

func getCurrentDate() map[string]string {
    d := make(map[string]string)
    curtm := time.Now().Local()
    curyear := fmt.Sprintf("%04d", curtm.Year())
    curmonth := fmt.Sprintf("%02d", curtm.Month())
    curday := fmt.Sprintf("%02d", curtm.Day())
    d["year"] = curyear
    d["month"] = curmonth
    d["day"] = curday
    return d
}

func renderFile(box packr.Box, infile string, outfile string) map[string] string {
    headmeta := make(map[string]string)
    blocks := splitFile(infile)
    dochtml := ""
    for _, block := range blocks {
        // For each block, remove its prefix empty newline
        // I am pullzed by golang's Trim, TrimPrefix, TrimLeft ...
        // so I write the both version, it works however ..
        block = bytes.TrimPrefix(block, []byte{0xa})
        block = bytes.Trim(block, "\n")

        // render article
        rendered := ""
        switch getBlockType(block) {
        case "TITLE":
            rendered = renderTitle(block)
        case "META":
            headmeta = getHeadline(block)
        case "LIST":
            rendered = renderList(bytes.Split(block, []byte{0xa}))
        case "CODE4":
            rendered = renderCode(block, 4)
        case "CODE8":
            rendered = renderCode(block, 8)
        default:
            rendered = renderPara(block)
        }
        dochtml += rendered + "\n"
    }

    // If the file has not been updated, then we return headmeta directly
    if !hasUpdated(infile, outfile + ".txt") {
        headmeta["latest"] = headmeta["latest"]
        _, fname := path.Split(infile)
        fmt.Printf("%s is remained unchanged, skipped!\n", fname)
        return headmeta
    }

    // set latest update date
    d := getCurrentDate()
    // year-month-day
    ymd := d["year"] + "-" + d["month"] + "-" + d["day"]
    // set default headline meta
    if len(headmeta) == 0 {
        fmt.Printf("[Warn] You do not supply any head meta information!\n")
        headmeta["title"] = filepath.Base(infile)
        headmeta["author"] = "Anonymous"
        headmeta["date"]  = ymd
    }
    headmeta["latest"] = ymd

    txt, _ := box.FindString("tpl/article.html")
    tpl, err := template.New("T").Parse(txt)
    if err != nil {
        log.Fatal("Cannot parse tpl/article.html!")
    }
    out, _ := os.Create(outfile)
    defer out.Close()
    articleData := struct {
        Style string
        Title string
        Date string
        Author string
        Latest string // The lasted update date
        Body template.HTML
    }{
        Style : pc.style,
        Title: headmeta["title"],
        Date: headmeta["date"],
        Author: headmeta["author"],
        Latest: headmeta["latest"],
        Body: template.HTML(dochtml)} // no new line after the right brace
    tpl.Execute(out, &articleData)

    // save a txt copy into the web, the user may append a ".txt" suffix to the web url
    // to view the original text content
    intxt, err := ioutil.ReadFile(infile)
    if err != nil {
        log.Fatal("Cannot read text file: %s\n", infile)
    }
    err = ioutil.WriteFile(outfile + ".txt", intxt, os.ModePerm)
    if err != nil {
        log.Fatal("Cannot write text file: %s\n", outfile + ".txt")
    }

    return headmeta
}

func expandPath(p string) (out string) {
    if strings.HasPrefix(p, "~") {
        usr, _ := user.Current()
        if len(p) > 1 {
            out = filepath.Join(usr.HomeDir, p[1:])
        } else {
            out = usr.HomeDir
        }
    } else {
        out, _ = filepath.Abs(p)
    }
    return out
}

func unpackResource(box packr.Box, unpack2dir string) {
    if _, err := os.Stat(unpack2dir); os.IsNotExist(err) {
        os.MkdirAll(unpack2dir, os.ModePerm)
    }
    resource := [...]string{"normalize.css", "pigger.css", "prism.css", "prism.js", "site.html"}
    cssdir := filepath.Join(unpack2dir, "css"); os.Mkdir(cssdir, os.ModePerm)
    jsdir := filepath.Join(unpack2dir, "js"); os.Mkdir(jsdir, os.ModePerm)
    tpldir := filepath.Join(unpack2dir, "tpl"); os.Mkdir(tpldir, os.ModePerm)
    for _, f := range resource {
        if strings.HasSuffix(f, ".css") {
            fout, _ := os.Create(filepath.Join(cssdir, f))
            txt, _ := box.FindString("css/" + f)
            fout.WriteString(txt)
        } else if strings.HasSuffix(f, ".js") {
            fout, _ := os.Create(filepath.Join(jsdir, f))
            txt, _ := box.FindString("js/" + f)
            fout.WriteString(txt)
        } else if strings.HasSuffix(f, ".html") {
            fout, _ := os.Create(filepath.Join(tpldir, f))
            txt, _ := box.FindString("tpl/" + f)
            fout.WriteString(txt)
        }
    }
}

func isPiggerSite(sitedir string) bool {
    // fmt.Printf("Build all files ...\n")
    // check if the current directory is a pigger project
    piggerconf := expandPath(filepath.Join(sitedir, "posts", "pigger.json"))
    if _, err := os.Stat(piggerconf); os.IsNotExist(err) {
        return false
    } else {
        return true
    }
}

func getFileHash(path string) map[string]string {
    data, err := ioutil.ReadFile(path)
    if err != nil {
        log.Fatal("Cannot open file %s to calculate hash!", path)
    }
    hash := make(map[string]string)
    // From https://gist.github.com/sergiotapia/8263278
    hasher := md5.New()
    hasher.Write(data)
    hash["md5"] = hex.EncodeToString(hasher.Sum(nil))
    return hash
}

func hasUpdated(oldfile string, newfile string) bool {
    if _, err := os.Stat(oldfile); os.IsNotExist(err) {
        fmt.Printf("Cannot find the file %s!\n", oldfile)
        return false
    }
    if _, err := os.Stat(newfile); os.IsNotExist(err) {
        // fmt.Printf("Cannot find the file %s!\n", newfile)
        // if no newfile is found, then the newfile should be regarded to be updated
        return true
    }
    oldmd5 := getFileHash(oldfile)["md5"]
    newmd5 := getFileHash(newfile)["md5"]
    if oldmd5 != newmd5 {
        return true
    } else {
        return false
    }
}

func main() {
    // pack static resources
    box := packr.NewBox("./etc")
    // set cmd argument options
    var outbase string
    flag.StringVar(&outbase, "o", "", "(optional) The output directory.")
    var cutoff bool
    flag.BoolVar(&cutoff, "x", false, "(optional) Cut off css and js files.")
    var style string
    flag.StringVar(&style, "style", "", "(optional) Specify a remote style root directory.")
    help := flag.Bool("h", false, "(optional) Show this help.")
    flag.Usage = func() {
        fmt.Printf("Usage: %s [[OPTIONS] <infile>]|[ACTIONS PARAMS]\nOPTIONS:\n", os.Args[0])
        flag.PrintDefaults()
        fmt.Printf("ACTIONS:\n")
        fmt.Printf("  build: Build all files\n")
        fmt.Printf("  new <sitename>: Create a new site\n")
        fmt.Printf("  update [style]: Update stuffs for pigger site\n")
        fmt.Printf("         style: update embedded style files such as css, js etc.\n")
    }
    flag.Parse()
    // check cmd args
    if *help || flag.NArg() == 0 {
        flag.Usage()
        os.Exit(0)
    }
    switch flag.Arg(0) {
    case "build":
        sitedir := expandPath(".")
        if !isPiggerSite(sitedir) {
            fmt.Printf("Not a pigger site, if it does is, please run this command in the pigger root!\n")
            os.Exit(1)
        }
        // Prepare all articles
        var articles []string
        if tmp, err := filepath.Glob(filepath.Join(sitedir, "*.txt")); err == nil {
            articles = append(articles, tmp...)
        } else {
            log.Fatal(err)
        }
        if tmp, err := filepath.Glob(filepath.Join(sitedir, "home", "*.txt")); err == nil {
            articles = append(articles, tmp...)
        } else {
            log.Fatal(err)
        }

        // render all articles
        post := make(map[string]postmeta)
        for _, article := range articles {
            barename := strings.TrimSuffix(filepath.Base(article), ".txt")
            outdir := filepath.Join(sitedir, "posts", barename)
            if _, err := os.Stat(outdir); os.IsNotExist(err) {
                os.Mkdir(outdir, os.ModePerm)
            }
            infile := article
            outfile := filepath.Join(outdir, "index.html")

            // set style
            pc.imgin_ = filepath.Dir(infile)
            pc.imgout_ = filepath.Join(outdir, "images")
            pc.style = "../pigger"

            headmeta := renderFile(box, infile, outfile)

            // metainfo for article
            // Several things to notice:
            // 1) Note that strings.TrimLeft is really tricky,
            // it may does not work as expected, for example:
            //  s := "refs/tags/account"
            //  tag := strings.TrimLeft(s, "refs/tags")
            // the code above will return "ccount".
            // What the fuck? See https://stackoverflow.com/questions/29187086/why-trimleft-doesnt-work-as-expected
            // for more details. Here we use strings.TrimPrefix indestead.
            // 2) Here I use relative path to link files, but the path sepeartor
            // in windows is slash. As a result, we should use filepath.ToSlash to make the
            // path canonical.
            // 3) When trim out the sitedir prefix from input or output file, there remains
            // a path sepeartor in the first location, we should remove it
            // 4) To avoid link is escaped in the template, we should use template.URL.
            relin := filepath.ToSlash(strings.TrimPrefix(infile, sitedir)[1:])
            relout := filepath.ToSlash(strings.TrimPrefix(outfile, sitedir)[1:])
            // fmt.Printf("sitedir: %s\n", sitedir)
            // fmt.Printf("infile: %s outfile: %s\n", infile, outfile)
            // fmt.Printf("in: %s out: %s headmeta: %v\n", relin, relout, headmeta)
            post[relin] = postmeta{Title: headmeta["title"], Date: headmeta["date"], Author: headmeta["author"], Link: template.URL(relout), Latest: headmeta["latest"]}
        }

        // create site index file(not index.html in case that user want to have their own
        // home page)
        siteindex, err := os.Create(filepath.Join(sitedir, "site.html"))
        defer siteindex.Close()
        tpl, err := template.ParseFiles(filepath.Join(sitedir, "posts", "pigger", "tpl", "site.html"))
        if err != nil {
            log.Fatal("Cannot parse site.html template!")
        }
        postitems := make([]postmeta, 0)
        for _, v := range post {
            postitems = append(postitems, v)
        }
        sort.Slice(postitems, func(i, j int) bool {
            return postitems[i].Date > postitems[j].Date
        })

        // check if there is migration
        migration := filepath.Join(sitedir, "migration", "index.json")
        if data, err := ioutil.ReadFile(migration); err == nil {
            mig := make([]postmeta, 0)
            jsoniter.Unmarshal(data, &mig)
            for idx, v := range(mig) {
                mig[idx].Link = template.URL(filepath.Join("migration", string(v.Link)))
            }
            postitems = append(postitems, mig...)
        }
        tpl.Execute(siteindex, &postitems)

        // write posts metainfo into json file: pigger.json
        // just for migration purpose
        jstr, err := jsoniter.Marshal(post)
        if err != nil {
            log.Fatal("Cannot marshal post!\n")
        }
        jfile, err := os.OpenFile(filepath.Join(sitedir, "posts", "pigger.json"), os.O_WRONLY, 0600)
        defer jfile.Close()
        if err != nil {
            log.Fatal("cannot open pigger.json");
        }
        if _, err = jfile.WriteString(string(jstr)); err != nil {
            log.Fatal(err)
        }
    case "new":
        if flag.NArg() != 2 {
            flag.Usage()
            log.Fatal("You forget input the name for the site, see the help above.\n")
        }
        sitedir := expandPath(flag.Arg(1))
        fmt.Printf("Create new site %s ...\n", sitedir)
        if _, err := os.Stat(sitedir); os.IsNotExist(err) {
            // Create draft directory
            os.MkdirAll(filepath.Join(sitedir, "draft"), os.ModePerm);

            // Create home directory
            os.MkdirAll(filepath.Join(sitedir, "home"), os.ModePerm);

            // Create .gitignore file for draft in site directory
            gitignore, err := os.Create(filepath.Join(sitedir, ".gitignore"))
            defer gitignore.Close()
            if err != nil {
                fmt.Printf("Cannot create .gitignore file in %s!\n", sitedir)
                log.Fatal(err)
            } else {
                gitignore.WriteString("draft\n")
            }

            // unpackResource will create the dir if it does not exist
            unpackResource(box, filepath.Join(sitedir, "posts", "pigger"))
            // create pigger configuration pigger.json
            piggerconf, err := os.Create(filepath.Join(sitedir, "posts", "pigger.json"))
            defer piggerconf.Close()
            if err != nil {
                log.Fatal("Cannot create pigger config file!\n")
            }
            fmt.Printf("Good! The new site is created successfully and could be found at %s!\n", sitedir)
        } else {
            fmt.Printf("The site is already there.\n")
        }
    case "update":
        if flag.NArg() != 2 {
            flag.Usage()
            log.Fatal("What do you want to update?\n")
        }
        switch flag.Arg(1) {
        case "style":
            if !isPiggerSite(expandPath(".")) {
                fmt.Printf("Not a pigger site, if it does is, please run this command in the pigger root!\n")
                os.Exit(1)
            } else {
                unpackResource(box, filepath.Join(expandPath("."), "posts", "pigger"))
            }
        default:
            log.Fatal("Unknown update option!\n")
        }
    default:
        infile := expandPath(flag.Arg(0))
        // prepare input and output
        _, fname := path.Split(infile)
        if path.Ext(fname) != ".txt" {
            fmt.Printf("Pigger only deals with text file (with a '.txt' suffix).\n")
            os.Exit(1)
        }
        // test if input file is exist
        if _, err := os.Stat(infile); os.IsNotExist(err) {
            log.Fatal("Input file is not exist!\n")
        }
        barename := strings.TrimSuffix(fname, path.Ext(fname))
        if outbase == "" {
            outbase, _ = filepath.Abs(".")
        } else {
            outbase = expandPath(outbase)
        }
        if _, err := os.Stat(outbase); os.IsNotExist(err) {
            os.MkdirAll(outbase, os.ModePerm)
        }
        outdir := filepath.Join(outbase, barename);os.Mkdir(outdir, os.ModePerm)
        outfile := filepath.Join(outdir, "index.html")

        // unpack static resources
        if cutoff {
            if style == "" {
                pc.style = "../pigger"
                unpackResource(box, expandPath(filepath.Join(outdir, pc.style)))
            } else {
                pc.style = style
            }
        } else {
            unpackResource(box, outdir)
            pc.style = "."
        }
        // render file
        pc.imgout_ = filepath.Join(outdir, "images")
        pc.imgin_ = filepath.Dir(infile)
        renderFile(box, infile, outfile)
        fmt.Printf("Save file into %s\n", outfile)
    }
}
