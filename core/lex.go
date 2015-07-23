// Copyright 2015 by caixw, All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package core

import (
	"bytes"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

const eof = -1

type lexer struct {
	data  []byte
	line  int    // data所在的起始行数
	file  string // 源文件名称
	pos   int    // 当前位置
	width int    // 最后移动位置的大小
}

// line data在源文件中的起始行号
func newLexer(data []byte, line int, file string) *lexer {
	return &lexer{
		data: data,
		line: line,
		file: file,
	}
}

// 当前位置在源代码中的行号
func (l *lexer) lineNumber() int {
	return l.line + bytes.Count(l.data[:l.pos], []byte("\n"))
}

// 获取下一个字符。
// 可通过lexer.backup来撤消最后一次调用。
func (l *lexer) next() rune {
	if l.pos >= len(l.data) {
		return eof
	}

	r, w := utf8.DecodeRune(l.data[l.pos:])
	l.pos += w
	l.width = w
	return r
}

// 读取从当前位置到换行符|n之间的内容。首尾空格将被舍弃。
// 可通过lexer.backup来撤消最后一次调用。
func (l *lexer) nextLine() string {
	rs := []rune{}
	for {
		if l.pos >= len(l.data) { // 提前结束
			return strings.TrimSpace(string(rs))
		}

		r, w := utf8.DecodeRune(l.data[l.pos:])
		l.pos += w
		l.width += w
		rs = append(rs, r)
		if r == '\n' {
			return strings.TrimSpace(string(rs))
		}
	}
}

// 读取当前行内，到下一个空格之间的单词。首尾空格将被舍弃。
// 可通过lexer.backup来撤消最后一次调用。
func (l *lexer) nextWord() (str string, eol bool) {
	rs := []rune{}
	for {
		if l.pos >= len(l.data) { // 提前结束
			return strings.TrimSpace(string(rs)), true
		}

		r, w := utf8.DecodeRune(l.data[l.pos:])
		l.pos += w
		l.width += w
		rs = append(rs, r)
		if unicode.IsSpace(r) {
			return strings.TrimSpace(string(rs)), r == '\n'
		}
	}
}

// 撤消next/nextN/nextLine函数的最后次调用。指针指向执行这些函数之前的位置。
func (l *lexer) backup() {
	l.pos -= l.width
	l.width = 0
}

// 判断接下去的几个字符连接起来是否正好为word，若不匹配，则不移动指针。
func (l *lexer) match(word string) bool {
	if l.pos+len(word) >= len(l.data) {
		return false
	}

	for _, r := range word {
		rr, w := utf8.DecodeRune(l.data[l.pos:])
		if rr != r {
			l.pos -= l.width
			l.width = 0
			return false
		}

		l.pos += w
		l.width += w
	}
	return true
}

func (l *lexer) scan() (*doc, error) {
	d := &doc{}
	var err error

LOOP:
	for {
		switch {
		case l.match("@apiURL"):
			err = l.scanApiURL(d)
		case l.match("@apiMethods"):
			err = l.scanApiMethods(d)
		case l.match("@apiVersion"):
			err = l.scanApiVersion(d)
		case l.match("@apiGroup"):
			err = l.scanApiGroup(d)
		case l.match("@apiQuery"):
			err = l.scanApiQuery(d)
		case l.match("@apiRequest"):
			err = l.scanApiRequest(d)
		case l.match("@apiStatus"):
			err = l.scanApiStatus(d)
		case l.match("@api"): // 放最后
			err = l.scanApi(d)
		default:
			if eof == l.next() { // 去掉无用的字符。
				break LOOP
			}
		}

		if err != nil {
			return nil, err
		}
	}

	return d, nil
}

func (l *lexer) scanApiURL(d *doc) error {
	str := l.nextLine()
	if len(str) == 0 {
		return errors.New("apiURL参数不能为空")
	}

	d.URL = str
	return nil
}

func (l *lexer) scanApiMethods(d *doc) error {
	str := l.nextLine()
	if len(str) == 0 {
		return errors.New("apiMethod缺少参数")
	}

	d.Methods = str
	return nil
}

func (l *lexer) scanApiVersion(d *doc) error {
	str := l.nextLine()
	if len(str) == 0 {
		return errors.New("apiMethod缺少参数")
	}

	d.Version = str
	return nil
}

func (l *lexer) scanApiGroup(d *doc) error {
	str := l.nextLine()
	if len(str) == 0 {
		return errors.New("apiMethod缺少参数")
	}

	d.Group = str
	return nil
}

func (l *lexer) scanApiQuery(d *doc) error {
	p, err := l.scanApiParam()
	if err != nil {
		return err
	}

	d.Queries = append(d.Queries, p)
	return nil
}

func (l *lexer) scanApiRequest(d *doc) error {
	r := &request{
		Type:     l.nextLine(),
		Headers:  map[string]string{},
		Params:   []*param{},
		Examples: []*example{},
	}

LOOP:
	for {
		switch {
		case l.match("@apiHeader"):
			key, eol := l.nextWord()
			if eol {
				return errors.New("apiHeader缺少value")
			}
			val := l.nextLine()
			if len(val) == 0 {
				return errors.New("apiHeader缺少value")
			}
			r.Headers[key] = val
		case l.match("@apiParam"):
			p, err := l.scanApiParam()
			if err != nil {
				return err
			}
			r.Params = append(r.Params, p)
		case l.match("@apiExample"):
			e, err := l.scanApiExample()
			if err != nil {
				return err
			}
			r.Examples = append(r.Examples, e)
		default:
			if eof == l.next() { // 去掉无用的字符。
				break LOOP
			}
		}
	}

	d.Request = r
	return nil
}

func (l *lexer) scanApiStatus(d *doc) error {
	status := &status{
		Headers:  map[string]string{},
		Params:   []*param{},
		Examples: []*example{},
	}

	var eol bool
	status.Code, eol = l.nextWord()
	if eol {
		return errors.New("apiStatus缺少必要的参数")
	}
	status.Type, eol = l.nextWord()
	if eol {
		return errors.New("apiStatus缺少必要的参数")
	}
	status.Summary = l.nextLine()

LOOP:
	for {
		switch {
		case l.match("@apiHeader"):
			key, eol := l.nextWord()
			if eol {
				return errors.New("apiHeader缺少value")
			}
			val := l.nextLine()
			if len(val) == 0 {
				return errors.New("apiHeader缺少value")
			}
			status.Headers[key] = val
		case l.match("@apiParam"):
			p, err := l.scanApiParam()
			if err != nil {
				return err
			}
			status.Params = append(status.Params, p)
		case l.match("@apiExample"):
			e, err := l.scanApiExample()
			if err != nil {
				return err
			}
			status.Examples = append(status.Examples, e)
		default:
			if eof == l.next() { // 去掉无用的字符。
				break LOOP
			}
		}
	}

	return nil
}

func (l *lexer) scanApiExample() (*example, error) {
	e := &example{}
	var eol bool

	e.Type, eol = l.nextWord()
	if eol {
		return nil, errors.New("@apiExample缺少参数")
	}

	e.Code = l.nextLine()
	for {
		line := l.nextLine()
		if strings.Index(line, "@api") >= 0 {
			l.backup()
			break
		}
		e.Code += line
	}
	return e, nil
}

func (l *lexer) scanApiParam() (*param, error) {
	p := &param{}
	var eol bool
	for {
		switch {
		case len(p.Name) == 0:
			p.Name, eol = l.nextWord()
		case len(p.Type) == 0:
			p.Name, eol = l.nextWord()
		case !p.Optional && l.match("optional"):
			p.Optional = true
		default:
			p.Description = l.nextLine()
			eol = true
		}

		if eol {
			return p, nil
		}
	}
}

func (l *lexer) scanApi(d *doc) error {
	str := l.nextLine()
	if len(str) == 0 {
		return errors.New("api第一个参数不能为空")
	}
	d.Summary = str

	str = l.nextLine()

	// 有@api字符串，应该是另一个语句，回退最后次nextLine操作。
	if strings.Index(str, "@api") >= 0 {
		l.backup()
		return nil
	}
	d.Description = str
	return nil
}
