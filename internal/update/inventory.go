package update

import (
	"encoding/binary"
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode/utf16"
)

const InventoryName = "uninstall-files.ini"

// UninstallInventory is data for the NSIS uninstaller, never executable script.
// Files come first, then directories from children to parents. The inventory
// itself is removed by NSIS after closing it, and NSIS removes Uninstall.exe.
func UninstallInventory(files map[string]string) (string, error) {
	paths := packagePaths{}
	for _, reserved := range []string{InventoryName, "Uninstall.exe", ManifestName} {
		if err := paths.add(reserved, false); err != nil {
			return "", err
		}
	}
	names := []string{ManifestName}
	dirs := map[string]bool{}
	for name := range files {
		if name == ManifestName || name == InventoryName {
			continue
		}
		if err := paths.add(name, false); err != nil {
			return "", err
		}
		// Leave room for the record prefix, quotes and CRLF in NSIS's default
		// 1024-character string buffer. The full install path is checked by NSIS.
		if len(utf16.Encode([]rune(name))) > 800 {
			return "", fmt.Errorf("uninstall path is too long: %s", name)
		}
		names = append(names, name)
		for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
			dirs[dir] = true
		}
	}
	sort.Strings(names)
	var directories []string
	for dir := range dirs {
		directories = append(directories, dir)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(directories)))
	if len(names)+len(directories) > 50000 {
		return "", fmt.Errorf("uninstall inventory is too large")
	}
	var out strings.Builder
	fmt.Fprintf(&out, "[inventory]\r\nversion=1\r\ncount=%d\r\n", len(names)+len(directories))
	for i, name := range names {
		fmt.Fprintf(&out, "%d=\"F%s\"\r\n", i, strings.ReplaceAll(name, "/", "\\"))
	}
	for i, dir := range directories {
		fmt.Fprintf(&out, "%d=\"D%s\"\r\n", len(names)+i, strings.ReplaceAll(dir, "/", "\\"))
	}
	return out.String(), nil
}

// Windows INI APIs read Unicode reliably when the file has a UTF-16LE BOM.
// The installer writes the same encoding using FileWriteUTF16LE /BOM.
func inventoryBytes(text string) []byte {
	units := utf16.Encode([]rune(text))
	data := make([]byte, 2+2*len(units))
	binary.LittleEndian.PutUint16(data, 0xfeff)
	for i, u := range units {
		binary.LittleEndian.PutUint16(data[2+2*i:], u)
	}
	return data
}
