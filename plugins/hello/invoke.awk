function fail(message) {
  failed = 1
  print "alter-hello: invalid invoke JSON: " message > "/dev/stderr"
  exit 2
}

function skip_space() {
  while (pos <= length(source) && substr(source, pos, 1) ~ /[[:space:]]/) {
    pos++
  }
}

function parse_string(    start, decoded, c, escaped, hex) {
  if (substr(source, pos, 1) != "\"") {
    fail("expected string")
  }
  pos++
  start = pos
  decoded = ""
  while (pos <= length(source)) {
    c = substr(source, pos, 1)
    if (c == "\"") {
      string_raw = substr(source, start, pos - start)
      pos++
      return decoded
    }
    if (c == "\\") {
      pos++
      escaped = substr(source, pos, 1)
      if (escaped == "\"" || escaped == "\\" || escaped == "/") {
        decoded = decoded escaped
      } else if (escaped == "b") {
        decoded = decoded "\b"
      } else if (escaped == "f") {
        decoded = decoded "\f"
      } else if (escaped == "n") {
        decoded = decoded "\n"
      } else if (escaped == "r") {
        decoded = decoded "\r"
      } else if (escaped == "t") {
        decoded = decoded "\t"
      } else if (escaped == "u") {
        hex = substr(source, pos + 1, 4)
        if (hex !~ /^[0-9A-Fa-f]{4}$/) {
          fail("invalid unicode escape")
        }
        decoded = decoded unicode_key_character(hex)
        pos += 4
      } else {
        fail("invalid string escape")
      }
    } else {
      if (c ~ /[[:cntrl:]]/) {
        fail("control character in string")
      }
      decoded = decoded c
    }
    pos++
  }
  fail("unterminated string")
}

function unicode_key_character(hex,    value) {
  value = hex_value(hex)
  if (value < 128) {
    return sprintf("%c", value)
  }
  return "?"
}

function hex_value(hex,    i, digit, value) {
  value = 0
  for (i = 1; i <= length(hex); i++) {
    digit = index("0123456789abcdef", tolower(substr(hex, i, 1))) - 1
    value = value * 16 + digit
  }
  return value
}

function parse_number(    rest) {
  rest = substr(source, pos)
  if (match(rest, /^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?/) != 1) {
    fail("invalid number")
  }
  pos += RLENGTH
}

function parse_literal(literal) {
  if (substr(source, pos, length(literal)) != literal) {
    fail("invalid literal")
  }
  pos += length(literal)
}

function parse_array(path,    array_index) {
  pos++
  skip_space()
  if (substr(source, pos, 1) == "]") {
    pos++
    return
  }
  array_index = 0
  while (1) {
    parse_value(path "[" array_index "]")
    array_index++
    skip_space()
    if (substr(source, pos, 1) == "]") {
      pos++
      return
    }
    if (substr(source, pos, 1) != ",") {
      fail("expected comma or array end")
    }
    pos++
  }
}

function parse_object(path,    key, child_path) {
  pos++
  skip_space()
  if (substr(source, pos, 1) == "}") {
    pos++
    return
  }
  while (1) {
    skip_space()
    key = parse_string()
    skip_space()
    if (substr(source, pos, 1) != ":") {
      fail("expected colon")
    }
    pos++
    child_path = path == "" ? key : path "." key
    parse_value(child_path)
    skip_space()
    if (substr(source, pos, 1) == "}") {
      pos++
      return
    }
    if (substr(source, pos, 1) != ",") {
      fail("expected comma or object end")
    }
    pos++
  }
}

function parse_value(path,    c, decoded) {
  skip_space()
  c = substr(source, pos, 1)
  if (path == "args.name" && c != "\"") {
    fail("args.name must be a string")
  }
  if (c == "{") {
    parse_object(path)
  } else if (c == "[") {
    parse_array(path)
  } else if (c == "\"") {
    decoded = parse_string()
    if (path == "tool") {
      tool_value = decoded
    }
    if (path == "args.name") {
      name_raw = string_raw
      found_name = 1
    }
  } else if (c == "t") {
    parse_literal("true")
  } else if (c == "f") {
    parse_literal("false")
  } else if (c == "n") {
    parse_literal("null")
  } else {
    parse_number()
  }
}

{
  source = source $0 "\n"
}

END {
  if (failed) {
    exit 2
  }
  pos = 1
  parse_value("")
  skip_space()
  if (pos <= length(source)) {
    fail("trailing content")
  }
  if (!found_name || name_raw == "") {
    name_raw = "world"
  }
  if (tool_value != "greet") {
    fail("unsupported tool")
  }
  printf "{\n  \"message\": \"hello, %s\",\n  \"plugin\": \"hello\"\n}\n", name_raw
}
