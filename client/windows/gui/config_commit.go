package main

import "net/http"

func commitGUIConfig(w http.ResponseWriter, cfg *AppConfig) bool {
	if err := saveConfig(cfg); err != nil {
		appendLog("[ERR] Не удалось сохранить защищённые настройки; изменения не записаны.")
		http.Error(w, "Не удалось сохранить настройки", http.StatusInternalServerError)
		return false
	}
	return true
}
