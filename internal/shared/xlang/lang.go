package xlang

import "maps"

func NewTranslatedString() TranslatedString {
	return make(map[string]string)
}

// TranslatedStringFromMap returns a TranslatedString populated from m.
func TranslatedStringFromMap(m map[string]string) TranslatedString {
	t := make(TranslatedString, len(m))
	for k, v := range m {
		t[k] = v
	}
	return t
}

type TranslatedString map[string]string

const DefaultLanguage = "en"

func (t TranslatedString) For(lang string) string {
	if v, ok := t[lang]; ok {
		return v
	}
	if v, ok := t[DefaultLanguage]; ok {
		return v
	}
	return ""
}

func (t TranslatedString) With(lang, text string) TranslatedString {
	newTranslations := make(map[string]string, len(t)+1)
	maps.Copy(newTranslations, t)
	newTranslations[lang] = text
	return newTranslations
}

func (t TranslatedString) AsMap() map[string]string {
	newTranslations := make(map[string]string, len(t))
	maps.Copy(newTranslations, t)
	return newTranslations
}
