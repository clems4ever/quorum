package fakegoogle

import "strings"

// texts holds the translations of the account chooser page.
var texts = map[string]map[string]string{
	"en": {
		"Title":       "Sign in – Google Accounts (demo)",
		"Heading":     "Choose an account",
		"ContinueTo":  "to continue to",
		"Badge":       "DEMO · fake Google sign-in",
		"Explanation": "This page imitates Google's account chooser so the app can be tried without real credentials. Click a profile to sign in instantly.",
		"Another":     "Use another account",
		"FullName":    "Full name",
		"Email":       "Email",
		"SignIn":      "Sign in",
		"Cancel":      "Cancel",
	},
	"fr": {
		"Title":       "Connexion – Comptes Google (démo)",
		"Heading":     "Choisissez un compte",
		"ContinueTo":  "pour continuer vers",
		"Badge":       "DÉMO · faux Google",
		"Explanation": "Cette page imite le sélecteur de compte de Google pour essayer l'application sans vrais identifiants. Cliquez sur un profil pour vous connecter immédiatement.",
		"Another":     "Utiliser un autre compte",
		"FullName":    "Nom complet",
		"Email":       "Adresse e-mail",
		"SignIn":      "Se connecter",
		"Cancel":      "Annuler",
	},
}

// language picks the first supported language of an Accept-Language header,
// in the order the browser lists them. English is the default.
func language(acceptLanguage string) string {
	for _, part := range strings.Split(acceptLanguage, ",") {
		tag, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		base, _, _ := strings.Cut(strings.ToLower(tag), "-")
		if _, ok := texts[base]; ok {
			return base
		}
	}
	return "en"
}
