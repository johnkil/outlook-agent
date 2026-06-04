package mstimezone

import (
	"sort"
	"strings"
)

var windowsToIANA = map[string]string{
	"dateline standard time":          "Etc/GMT+12",
	"utc-11":                          "Etc/GMT+11",
	"aleutian standard time":          "America/Adak",
	"hawaiian standard time":          "Pacific/Honolulu",
	"marquesas standard time":         "Pacific/Marquesas",
	"alaskan standard time":           "America/Anchorage",
	"utc-09":                          "Etc/GMT+9",
	"pacific standard time (mexico)":  "America/Tijuana",
	"utc-08":                          "Etc/GMT+8",
	"pacific standard time":           "America/Los_Angeles",
	"us mountain standard time":       "America/Phoenix",
	"mountain standard time (mexico)": "America/Chihuahua",
	"mountain standard time":          "America/Denver",
	"central america standard time":   "America/Guatemala",
	"central standard time":           "America/Chicago",
	"easter island standard time":     "Pacific/Easter",
	"central standard time (mexico)":  "America/Mexico_City",
	"canada central standard time":    "America/Regina",
	"sa pacific standard time":        "America/Bogota",
	"eastern standard time (mexico)":  "America/Cancun",
	"eastern standard time":           "America/New_York",
	"haiti standard time":             "America/Port-au-Prince",
	"cuba standard time":              "America/Havana",
	"us eastern standard time":        "America/Indiana/Indianapolis",
	"turks and caicos standard time":  "America/Grand_Turk",
	"paraguay standard time":          "America/Asuncion",
	"atlantic standard time":          "America/Halifax",
	"venezuela standard time":         "America/Caracas",
	"central brazilian standard time": "America/Cuiaba",
	"sa western standard time":        "America/La_Paz",
	"pacific sa standard time":        "America/Santiago",
	"newfoundland standard time":      "America/St_Johns",
	"tocantins standard time":         "America/Araguaina",
	"e. south america standard time":  "America/Sao_Paulo",
	"sa eastern standard time":        "America/Cayenne",
	"argentina standard time":         "America/Argentina/Buenos_Aires",
	"greenland standard time":         "America/Nuuk",
	"montevideo standard time":        "America/Montevideo",
	"magallanes standard time":        "America/Punta_Arenas",
	"saint pierre standard time":      "America/Miquelon",
	"bahia standard time":             "America/Bahia",
	"utc-02":                          "Etc/GMT+2",
	"mid-atlantic standard time":      "Atlantic/South_Georgia",
	"azores standard time":            "Atlantic/Azores",
	"cape verde standard time":        "Atlantic/Cape_Verde",
	"utc":                             "UTC",
	"coordinated universal time":      "UTC",
	"gmt standard time":               "Europe/London",
	"greenwich standard time":         "Atlantic/Reykjavik",
	"sao tome standard time":          "Africa/Sao_Tome",
	"morocco standard time":           "Africa/Casablanca",
	"w. europe standard time":         "Europe/Berlin",
	"central europe standard time":    "Europe/Budapest",
	"romance standard time":           "Europe/Paris",
	"central european standard time":  "Europe/Warsaw",
	"w. central africa standard time": "Africa/Lagos",
	"jordan standard time":            "Asia/Amman",
	"gtb standard time":               "Europe/Bucharest",
	"middle east standard time":       "Asia/Beirut",
	"egypt standard time":             "Africa/Cairo",
	"e. europe standard time":         "Europe/Chisinau",
	"syria standard time":             "Asia/Damascus",
	"west bank standard time":         "Asia/Hebron",
	"south africa standard time":      "Africa/Johannesburg",
	"fle standard time":               "Europe/Kyiv",
	"israel standard time":            "Asia/Jerusalem",
	"kaliningrad standard time":       "Europe/Kaliningrad",
	"sudan standard time":             "Africa/Khartoum",
	"libya standard time":             "Africa/Tripoli",
	"namibia standard time":           "Africa/Windhoek",
	"arabic standard time":            "Asia/Baghdad",
	"turkey standard time":            "Europe/Istanbul",
	"arab standard time":              "Asia/Riyadh",
	"belarus standard time":           "Europe/Minsk",
	"russian standard time":           "Europe/Moscow",
	"e. africa standard time":         "Africa/Nairobi",
	"iran standard time":              "Asia/Tehran",
	"arabian standard time":           "Asia/Dubai",
	"astrakhan standard time":         "Europe/Astrakhan",
	"azerbaijan standard time":        "Asia/Baku",
	"russia time zone 3":              "Europe/Samara",
	"mauritius standard time":         "Indian/Mauritius",
	"saratov standard time":           "Europe/Saratov",
	"georgian standard time":          "Asia/Tbilisi",
	"caucasus standard time":          "Asia/Yerevan",
	"afghanistan standard time":       "Asia/Kabul",
	"west asia standard time":         "Asia/Tashkent",
	"ekaterinburg standard time":      "Asia/Yekaterinburg",
	"pakistan standard time":          "Asia/Karachi",
	"qyzylorda standard time":         "Asia/Qyzylorda",
	"india standard time":             "Asia/Kolkata",
	"sri lanka standard time":         "Asia/Colombo",
	"nepal standard time":             "Asia/Kathmandu",
	"central asia standard time":      "Asia/Almaty",
	"bangladesh standard time":        "Asia/Dhaka",
	"omsk standard time":              "Asia/Omsk",
	"myanmar standard time":           "Asia/Yangon",
	"se asia standard time":           "Asia/Bangkok",
	"altai standard time":             "Asia/Barnaul",
	"w. mongolia standard time":       "Asia/Hovd",
	"north asia standard time":        "Asia/Krasnoyarsk",
	"n. central asia standard time":   "Asia/Novosibirsk",
	"tomsk standard time":             "Asia/Tomsk",
	"china standard time":             "Asia/Shanghai",
	"north asia east standard time":   "Asia/Irkutsk",
	"singapore standard time":         "Asia/Singapore",
	"w. australia standard time":      "Australia/Perth",
	"taipei standard time":            "Asia/Taipei",
	"ulaanbaatar standard time":       "Asia/Ulaanbaatar",
	"aus central w. standard time":    "Australia/Eucla",
	"transbaikal standard time":       "Asia/Chita",
	"tokyo standard time":             "Asia/Tokyo",
	"north korea standard time":       "Asia/Pyongyang",
	"korea standard time":             "Asia/Seoul",
	"yakutsk standard time":           "Asia/Yakutsk",
	"cen. australia standard time":    "Australia/Adelaide",
	"aus central standard time":       "Australia/Darwin",
	"e. australia standard time":      "Australia/Brisbane",
	"aus eastern standard time":       "Australia/Sydney",
	"west pacific standard time":      "Pacific/Port_Moresby",
	"tasmania standard time":          "Australia/Hobart",
	"vladivostok standard time":       "Asia/Vladivostok",
	"lord howe standard time":         "Australia/Lord_Howe",
	"bougainville standard time":      "Pacific/Bougainville",
	"russia time zone 10":             "Asia/Srednekolymsk",
	"magadan standard time":           "Asia/Magadan",
	"norfolk standard time":           "Pacific/Norfolk",
	"sakhalin standard time":          "Asia/Sakhalin",
	"central pacific standard time":   "Pacific/Guadalcanal",
	"russia time zone 11":             "Asia/Kamchatka",
	"new zealand standard time":       "Pacific/Auckland",
	"utc+12":                          "Etc/GMT-12",
	"fiji standard time":              "Pacific/Fiji",
	"kamchatka standard time":         "Asia/Kamchatka",
	"chatham islands standard time":   "Pacific/Chatham",
	"utc+13":                          "Etc/GMT-13",
	"tonga standard time":             "Pacific/Tongatapu",
	"samoa standard time":             "Pacific/Apia",
	"line islands standard time":      "Pacific/Kiritimati",
}

func IANALocationName(timeZone string) string {
	return windowsToIANA[strings.ToLower(strings.TrimSpace(timeZone))]
}

var preferredWindowsByIANA = map[string]string{
	"utc":            "UTC",
	"asia/kamchatka": "Russia Time Zone 11",
}

var canonicalWindowsNames = map[string]string{
	"utc":                             "UTC",
	"utc-11":                          "UTC-11",
	"utc-09":                          "UTC-09",
	"utc-08":                          "UTC-08",
	"utc-02":                          "UTC-02",
	"utc+12":                          "UTC+12",
	"utc+13":                          "UTC+13",
	"gmt standard time":               "GMT Standard Time",
	"us mountain standard time":       "US Mountain Standard Time",
	"us eastern standard time":        "US Eastern Standard Time",
	"sa pacific standard time":        "SA Pacific Standard Time",
	"pacific sa standard time":        "Pacific SA Standard Time",
	"sa western standard time":        "SA Western Standard Time",
	"sa eastern standard time":        "SA Eastern Standard Time",
	"e. south america standard time":  "E. South America Standard Time",
	"w. europe standard time":         "W. Europe Standard Time",
	"w. central africa standard time": "W. Central Africa Standard Time",
	"e. europe standard time":         "E. Europe Standard Time",
	"e. africa standard time":         "E. Africa Standard Time",
	"w. mongolia standard time":       "W. Mongolia Standard Time",
	"w. australia standard time":      "W. Australia Standard Time",
	"n. central asia standard time":   "N. Central Asia Standard Time",
	"cen. australia standard time":    "Cen. Australia Standard Time",
	"mid-atlantic standard time":      "Mid-Atlantic Standard Time",
	"gtb standard time":               "GTB Standard Time",
	"fle standard time":               "FLE Standard Time",
	"se asia standard time":           "SE Asia Standard Time",
	"aus central w. standard time":    "AUS Central W. Standard Time",
	"aus central standard time":       "AUS Central Standard Time",
	"aus eastern standard time":       "AUS Eastern Standard Time",
}

var ianaToWindows = buildIANAToWindows()

func WindowsLocationName(timeZone string) string {
	normalized := strings.ToLower(strings.TrimSpace(timeZone))
	if normalized == "" {
		return ""
	}
	if _, ok := windowsToIANA[normalized]; ok {
		return canonicalWindowsName(normalized)
	}
	return ianaToWindows[normalized]
}

func buildIANAToWindows() map[string]string {
	out := make(map[string]string, len(windowsToIANA))
	windowsKeys := make([]string, 0, len(windowsToIANA))
	for windows := range windowsToIANA {
		windowsKeys = append(windowsKeys, windows)
	}
	sort.Strings(windowsKeys)

	for _, windows := range windowsKeys {
		iana := windowsToIANA[windows]
		key := strings.ToLower(strings.TrimSpace(iana))
		if key == "" {
			continue
		}
		if _, exists := out[key]; !exists {
			out[key] = canonicalWindowsName(windows)
		}
	}
	for iana, windows := range ianaAliasesToWindows {
		key := strings.ToLower(strings.TrimSpace(iana))
		if key == "" {
			continue
		}
		if _, exists := out[key]; !exists {
			out[key] = canonicalWindowsName(windows)
		}
	}
	for iana, windows := range preferredWindowsByIANA {
		out[strings.ToLower(strings.TrimSpace(iana))] = canonicalWindowsName(windows)
	}
	return out
}

func canonicalWindowsName(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if canonical, ok := canonicalWindowsNames[normalized]; ok {
		return canonical
	}

	words := strings.Fields(normalized)
	for index, word := range words {
		words[index] = canonicalWindowsWord(word)
	}
	return strings.Join(words, " ")
}

func canonicalWindowsWord(word string) string {
	switch word {
	case "utc", "gmt", "sa", "us":
		return strings.ToUpper(word)
	}
	if strings.HasPrefix(word, "utc") {
		return strings.ToUpper(word)
	}
	if len(word) == 2 && strings.HasSuffix(word, ".") {
		return strings.ToUpper(word[:1]) + "."
	}
	if len(word) == 0 {
		return word
	}
	return capitalizeFirstAlphabetic(word)
}

func capitalizeFirstAlphabetic(word string) string {
	for index, char := range word {
		if char >= 'a' && char <= 'z' {
			return word[:index] + strings.ToUpper(word[index:index+1]) + word[index+1:]
		}
	}
	return word
}
