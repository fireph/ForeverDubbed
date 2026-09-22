"""Offline checks for the SQL-to-Lua data importer."""
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location(
    "importer", Path(__file__).resolve().parents[1] / "scripts/import-voiceover-races.py")
importer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(importer)


class ImportTests(unittest.TestCase):
    def test_schema_and_numeric_fields(self):
        sql = ("CREATE TABLE `extra` (`ID` int, `Race` int, `Sex` int, PRIMARY KEY (`ID`));\n"
               "INSERT INTO `extra` VALUES (17, 9, 1, 'ignored');")
        self.assertEqual(importer.parse_rows(sql, "extra", ("ID", "Race", "Sex")), {17: (9, 1)})
        for bad in (sql.replace("`Race`", "`Wrong`"), sql.replace("17, 9", "17, NULL"),
                    sql + "\nINSERT INTO `extra` VALUES (17, 1, 0, 'duplicate');",
                    sql.replace("INSERT INTO `extra`", "INSERT INTO `other`")):
            with self.assertRaises(ValueError):
                importer.parse_rows(bad, "extra", ("ID", "Race", "Sex"))

    def test_join_uses_extra_id_and_database_sex(self):
        displays = {115: (999, 0, 7), 176: (888, 0, 8), 1: (1, 0, 0),
                    2: (1, 0, 99), 3: (1, 0, 9), 4: (1, 0, 10)}
        rows, skipped = importer.join_rows(displays, {7: (3, 0), 8: (1, 1), 9: (9, 2), 10: (999, 0)})
        self.assertEqual(rows, {115: 6, 176: 3})
        self.assertEqual(skipped, {"no_extra_record": 1, "missing_extra_record": 1,
                                   "unsupported_identity": 2})
        output = importer.render(rows)
        self.assertIn('[6] = {race="Dwarf", gender="male"}', output)
        self.assertIn('[3] = {race="Human", gender="female"}', output)
        self.assertEqual(output, importer.render(dict(reversed(list(rows.items())))))


if __name__ == "__main__":
    unittest.main()
