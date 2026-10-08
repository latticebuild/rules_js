"""Which directories discovery visits, shared through the watched pnpm index.

Bazel's literal ignore declarations in REPO.bazel and .bazelignore, and the
bounded doublestar matching of inherited Gazelle directory exclusions.
"""

load("@bazel_skylib//lib:paths.bzl", "paths")

def repository_selection(ctx, root):
    """Read explicit Bazel exclusions without evaluating repository code.

    Args:
        ctx: Repository context that watches the selection files.
        root: Canonical main-workspace path.

    Returns:
        The REPO.bazel and .bazelignore sources, with their ignored directory
        patterns and literal paths.
    """
    sources = {}
    for name in ["REPO.bazel", ".bazelignore"]:
        file = root.get_child(name)
        ctx.watch(file)
        if file.exists and file.realpath != file:
            fail("Repository selection must be a regular file: %s" % file)
        sources[name] = ctx.read(file) if file.exists else ""
        if len(sources[name]) > 1024 * 1024:
            fail("Repository selection exceeds 1 MiB: %s" % file)
    return dict(sources, directories = ignored_directories(sources["REPO.bazel"]), paths = ignored_paths(sources[".bazelignore"]))

def ignored_paths(contents):
    """Match .bazelignore's literal, repository-relative file/directory paths.

    Args:
        contents: The .bazelignore text.

    Returns:
        Normalized repository-relative paths.
    """
    result = []
    for line in contents.splitlines():
        value = line.strip()
        if not value or value.startswith("#"):
            continue
        if any([character in value for character in "*?[\\".elems()]):
            fail(".bazelignore paths must be literal slash paths: %r" % value)
        clean = paths.normalize(value)
        if paths.is_absolute(clean) or clean == ".." or clean.startswith("../"):
            fail(".bazelignore path leaves the repository: %r" % value)
        result.append(clean)
    return result

def ignored_directories(contents):
    """Read the literal list accepted by Bazel's ignore_directories declaration.

    Args:
        contents: The REPO.bazel text.

    Returns:
        The declared directory patterns, or an empty list without a declaration.
    """
    tokens = _tokens(contents)
    calls = [token for token in tokens if token == ("word", "ignore_directories")]
    if len(calls) > 1:
        fail("REPO.bazel ignore_directories requires one literal call")
    depth = 0
    prefix = []
    for index, token in enumerate(tokens):
        if depth == 0 and token in [("symbol", "\n"), ("symbol", ";")]:
            prefix = []
            continue
        selected = token == ("word", "ignore_directories")
        assignment = len(prefix) == 2 and prefix[0][0] == "word" and prefix[1] == ("symbol", "=")
        if selected and (depth != 0 or (prefix and not assignment)):
            fail("REPO.bazel ignore_directories requires a standalone or assigned literal call")
        if depth == 0:
            prefix.append(token)
        if token in [("symbol", "("), ("symbol", "["), ("symbol", "{")]:
            depth += 1
        elif token in [("symbol", ")"), ("symbol", "]"), ("symbol", "}")]:
            depth -= 1
        if not selected:
            continue
        remaining = [item for item in tokens[index + 1:] if item != ("symbol", "\n")]
        if remaining[:2] != [("symbol", "("), ("symbol", "[")]:
            fail("REPO.bazel ignore_directories requires a literal string list")
        result = []
        end = None
        expect_string = True
        for offset, item in enumerate(remaining[2:]):
            if item == ("symbol", "]"):
                end = offset + 3
                break
            if expect_string and item[0] == "string":
                result.append(item[1])
                expect_string = False
            elif not expect_string and item == ("symbol", ","):
                expect_string = True
            else:
                fail("REPO.bazel ignore_directories requires a literal string list")
        if end != None and remaining[end:end + 1] == [("symbol", ",")]:
            end += 1
        if end == None or remaining[end:end + 1] != [("symbol", ")")]:
            fail("Malformed REPO.bazel ignore_directories declaration")
        return result
    return []

def _tokens(contents):
    result = []
    skip = 0
    quote = ""
    start = 0
    escaped = False
    raw_string = False
    comment = False
    word = ""
    for index, char in enumerate(contents.elems()):
        if index < skip:
            continue
        if comment:
            comment = char != "\n"
            if not comment:
                result.append(("symbol", "\n"))
        elif quote:
            if escaped:
                escaped = False
            elif char == "\\":
                escaped = True
            elif contents[index:].startswith(quote):
                value = contents[start:index]
                result.append(("string", value if raw_string else _unescape(value)))
                skip = index + len(quote)
                quote = ""
        elif char in "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_0123456789":
            word += char
        else:
            raw_string = word in ["r", "R"] and char in ["'", '"']
            if word and not raw_string:
                result.append(("word", word))
            word = ""
            if char == "#":
                comment = True
            elif char in ["'", '"']:
                quote = char * 3 if contents[index:].startswith(char * 3) else char
                start = index + len(quote)
                skip = start
            elif char.strip() or char == "\n":
                result.append(("symbol", char))
    if quote:
        fail("Unterminated string in REPO.bazel")
    if word:
        result.append(("word", word))
    return result

def _unescape(value):
    # Starlark accepts Python-style literals, including octal/hex and raw
    # strings. JSON decoding alone silently changes that language boundary.
    result = []
    skip = 0
    simple = {"\n": "", '"': '"', "'": "'", "\\": "\\", "a": "\a", "b": "\b", "f": "\f", "n": "\n", "r": "\r", "t": "\t", "v": "\v"}
    for index, char in enumerate(value.elems()):
        if index < skip:
            continue
        if char != "\\":
            result.append(char)
            continue
        if index + 1 == len(value):
            fail("Truncated escape in REPO.bazel string")
        escape = value[index + 1]
        skip = index + 2
        if escape in simple:
            result.append(simple[escape])
        elif escape in "01234567":
            digits = escape
            for offset in [2, 3]:
                if index + offset >= len(value) or value[index + offset] not in "01234567":
                    break
                digits += value[index + offset]
            skip = index + 1 + len(digits)
            number = int(digits, 8)
            if number > 255:
                fail("Invalid octal escape in REPO.bazel string")
            result.append(_codepoint(number))
        elif escape in ["x", "u", "U"]:
            count = {"U": 8, "u": 4, "x": 2}[escape]
            digits = value[index + 2:index + 2 + count]
            if len(digits) != count or any([digit not in "0123456789abcdefABCDEF" for digit in digits.elems()]):
                fail("Invalid hexadecimal escape in REPO.bazel string")
            result.append(_codepoint(int(digits, 16)))
            skip = index + 2 + count
        else:
            result.append("\\" + escape)
    return "".join(result)

def _codepoint(number):
    if number > 0x10ffff or (0xd800 <= number and number <= 0xdfff):
        fail("Invalid Unicode escape in REPO.bazel string")
    values = [number] if number < 0x10000 else [0xd800 + (number - 0x10000) // 1024, 0xdc00 + (number - 0x10000) % 1024]
    encoded = []
    for value in values:
        encoded.append("\\u" + "".join(["0123456789abcdef"[(value // power) % 16] for power in [4096, 256, 16, 1]]))
    return json.decode('"' + "".join(encoded) + '"')

_UTF8_TWO = json.decode('"\\u0080"')[0]
_UTF8_THREE = json.decode('"\\u0800"')[0]
_UTF8_FOUR = json.decode('"\\ud800\\udc00"')[0]

def compile_patterns(pattern):
    """Compile a pattern into alternatives of tokens, without recursive matching.

    Args:
        pattern: Repository-relative doublestar pattern from a Gazelle exclusion.

    Returns:
        Tokenized alternatives that can be reused across the bounded source walk.
    """
    if not pattern or pattern.startswith("/") or len(pattern) > 4096:
        fail("Invalid Gazelle exclusion pattern: %r" % pattern)
    pending = [pattern]
    expanded = []
    for _ in range(4096):
        if not pending:
            break
        value = pending.pop()
        opening = -1
        closing = -1
        depth = 0
        escaped = False
        in_class = False
        commas = []
        for i, char in enumerate(value.elems()):
            if escaped:
                escaped = False
            elif char == "\\":
                escaped = True
            elif in_class:
                in_class = char != "]"
            elif char == "[":
                in_class = True
            elif char == "{":
                if depth == 0:
                    opening = i
                depth += 1
            elif char == "}":
                depth -= 1
                if depth < 0:
                    fail("Unmatched brace in Gazelle exclusion: %r" % pattern)
                if depth == 0:
                    closing = i
                    break
            elif char == "," and depth == 1:
                commas.append(i)
        if depth or (opening >= 0 and closing < 0):
            fail("Unmatched brace in Gazelle exclusion: %r" % pattern)
        if opening < 0:
            expanded.append(_pattern_tokens(value))
            continue
        previous = opening
        for end in commas + [closing]:
            pending.append(value[:opening] + value[previous + 1:end] + value[closing + 1:])
            previous = end
    if pending:
        fail("Gazelle exclusion exceeds 4096 brace expansions: %r" % pattern)
    return expanded

def _pattern_tokens(pattern):
    chars = _characters(pattern)
    tokens = []
    skip = 0
    start = True
    for i, char in enumerate(chars):
        if i < skip:
            continue
        remaining = "".join(chars[i:])
        if remaining in ["/**", "/**/"]:
            tokens.append(("tail", remaining.endswith("/"), True))
            break
        if char == "\\":
            if i + 1 == len(chars):
                fail("Trailing escape in Gazelle exclusion: %r" % pattern)
            token = ("literal", chars[i + 1])
            start = chars[i + 1] == "/"
            skip = i + 2
        elif char == "*":
            if start and remaining.startswith("**") and (len(remaining) == 2 or remaining[2] == "/"):
                token = ("all" if len(remaining) == 2 else "directories", "")
                skip = i + 3
                start = len(remaining) > 2
            else:
                token = ("*", "")
                start = False
        elif char == "[":
            token, skip = _class(chars, i, pattern)
            start = False
        else:
            token = ("?", "") if char == "?" else ("literal", char)
            start = char == "/"
        tokens.append((token[0], token[1], remaining in ["*", "**", "**/"]))
    return tokens

def _class(chars, opening, pattern):
    start = opening + 1
    negated = start < len(chars) and chars[start] in ["!", "^"]
    if negated:
        start += 1
    if start == len(chars) or chars[start] == "]":
        fail("Empty character class in Gazelle exclusion: %r" % pattern)
    ranges = []
    previous = None
    skip = start
    for i in range(start, len(chars)):
        if i < skip:
            continue
        char = chars[i]
        if char == "]":
            return ("not" if negated else "class", ranges), i + 1
        if char == "-" and previous != None and i + 1 < len(chars) and chars[i + 1] != "]":
            end = i + 1
            if chars[end] == "\\":
                end += 1
            if end == len(chars):
                break
            ranges.append((previous, chars[end]))
            previous = None
            skip = end + 1
        else:
            if char == "\\":
                if i + 1 == len(chars):
                    break
                char = chars[i + 1]
                skip = i + 2
            ranges.append((char, char))
            previous = char
    fail("Unclosed character class in Gazelle exclusion: %r" % pattern)

def matches(patterns, path):
    """Match a whole path, including Unicode characters and escaped separators.

    Args:
        patterns: Alternatives returned by compile_patterns.
        path: Repository-relative slash path to match.

    Returns:
        Whether any complete alternative matches the path.
    """
    chars = _characters(path)
    for tokens in patterns:
        positions = {0: True}
        for kind, contents, empty in tokens:
            following = {}
            for position in positions:
                # Doublestar accepts only specific suffixes once the path ends;
                # chaining otherwise-empty **/ tokens changes that contract.
                if position == len(chars):
                    if empty:
                        return True
                    continue
                if kind == "all":
                    following[len(chars)] = True
                elif kind == "tail":
                    if position == len(chars) or (chars[position] == "/" and (not contents or chars[-1] == "/")):
                        following[len(chars)] = True
                elif kind in ["*", "directories"]:
                    following[position] = True
                    for end in range(position + 1, len(chars) + 1):
                        if kind == "*":
                            if chars[end - 1] == "/":
                                break
                            following[end] = True
                        elif chars[end - 1] == "/":
                            following[end] = True
                elif position < len(chars):
                    char = chars[position]
                    if kind == "literal":
                        accepts = char == contents
                    elif kind == "?":
                        accepts = char != "/"
                    else:
                        accepts = any([low <= char and char <= high for low, high in contents])
                        if kind == "not":
                            accepts = not accepts
                    if accepts:
                        following[position + 1] = True
            positions = following
            if not positions:
                break
        if len(chars) in positions:
            return True
    return False

def _characters(value):
    # Bazel strings index UTF-8 bytes, while doublestar's ? and classes consume
    # Unicode code points. Group complete encodings without a host subprocess.
    result = []
    skip = 0
    for i, byte in enumerate(value.elems()):
        if i < skip:
            continue
        width = 4 if byte >= _UTF8_FOUR else 3 if byte >= _UTF8_THREE else 2 if byte >= _UTF8_TWO else 1
        result.append(value[i:i + width])
        skip = i + width
    return result
