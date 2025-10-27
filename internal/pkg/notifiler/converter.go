package notifiler

import "strings"

func AdjustMarkdownLinksToSlackWebhookFormat(in string) string {
	if in == "" {
		return in
	}

	r := []rune(in)
	var b strings.Builder
	n := len(r)

	for i := 0; i < n; {
		if i+2 < n && string(r[i:i+3]) == "```" {
			start := i
			i += 3
			for i < n {
				if i+2 < n && string(r[i:i+3]) == "```" {
					i += 3
					break
				}
				i++
			}
			b.WriteString(string(r[start:i]))
			continue
		}

		if r[i] == '`' {
			start := i
			i++
			for i < n && r[i] != '`' {
				if r[i] == '\\' && i+1 < n {
					i++
				}
				i++
			}
			if i < n {
				i++
			}
			b.WriteString(string(r[start:i]))
			continue
		}

		if r[i] == '[' {
			if url, text, end, ok := parseLink(r, i, n); ok {
				b.WriteString("<")
				b.WriteString(url)
				b.WriteString("|")
				b.WriteString(text)
				b.WriteString(">")
				i = end
				continue
			}
		}

		b.WriteRune(r[i])
		i++
	}

	return b.String()
}

func parseLink(r []rune, start, n int) (url, text string, end int, ok bool) {
	if start >= n || r[start] != '[' {
		return "", "", start, false
	}

	i := start + 1
	for i < n && r[i] != ']' && r[i] != '\n' {
		i++
	}
	if i >= n || r[i] != ']' {
		return "", "", start, false
	}

	text = string(r[start+1 : i])
	i++

	if i >= n || r[i] != '(' {
		return "", "", start, false
	}
	i++

	urlStart := i
	for i < n && r[i] != ')' && r[i] != '\n' {
		i++
	}
	if i >= n || r[i] != ')' {
		return "", "", start, false
	}

	url = string(r[urlStart:i])
	i++

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "", "", start, false
	}

	return url, text, i, true
}
