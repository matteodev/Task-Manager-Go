package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// MODEL
type Task struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	LastUpdated time.Time `json:"last_updated"`
	Done        bool      `json:"done"`
}

// STORAGE
var tasks []Task
var dataFile = "tasks.json"

// PAGE DATA
type HomePageData struct {
	Titolo    string
	Tasks     []Task
	AlertType string
	Message   string
}

type EditPageData struct {
	Titolo string
	Task   *Task
}

// FLASH HELPERS
func setFlash(w http.ResponseWriter, alertType, message string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "flash_type",
		Value:    url.QueryEscape(alertType),
		Path:     "/",
		HttpOnly: true,
		MaxAge:   10,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "flash_message",
		Value:    url.QueryEscape(message),
		Path:     "/",
		HttpOnly: true,
		MaxAge:   10,
	})
}

func getFlash(w http.ResponseWriter, r *http.Request) (string, string) {
	typeCookie, err1 := r.Cookie("flash_type")
	msgCookie, err2 := r.Cookie("flash_message")

	if err1 != nil || err2 != nil {
		return "", ""
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "flash_type",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "flash_message",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	alertType, _ := url.QueryUnescape(typeCookie.Value)
	message, _ := url.QueryUnescape(msgCookie.Value)

	return alertType, message
}

// HOME PAGE
func home(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("template/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	alertType, message := getFlash(w, r)

	data := HomePageData{
		Titolo:    "Task List",
		Tasks:     tasks,
		AlertType: alertType,
		Message:   message,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// NEW TASK PAGE
func addTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Metodo non supportato", http.StatusMethodNotAllowed)
		return
	}

	tmpl, err := template.ParseFiles("template/edit.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := EditPageData{
		Titolo: "Create New Task",
		Task:   nil,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// EDIT TASK PAGE
func editTaskPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Metodo non supportato", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimSuffix(r.URL.Path, "/edit")
	id, err := extractIDFromPath(path)
	if err != nil {
		http.Error(w, "ID non valido", http.StatusBadRequest)
		return
	}

	var foundTask *Task
	for i := range tasks {
		if tasks[i].ID == id {
			foundTask = &tasks[i]
			break
		}
	}

	if foundTask == nil {
		http.Error(w, "Task non trovata", http.StatusNotFound)
		return
	}

	tmpl, err := template.ParseFiles("template/edit.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := EditPageData{
		Titolo: "Edit Task",
		Task:   foundTask,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// CREATE TASK FROM FORM
func createTask(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		setFlash(w, "danger", "Errore lettura form")
		http.Redirect(w, r, "/tasks/new", http.StatusSeeOther)
		return
	}

	task := Task{
		Title:       strings.TrimSpace(r.FormValue("title")),
		Description: strings.TrimSpace(r.FormValue("description")),
		Done:        false,
	}

	err = validateTask(task)
	if err != nil {
		setFlash(w, "danger", err.Error())
		http.Redirect(w, r, "/tasks/new", http.StatusSeeOther)
		return
	}

	now := time.Now()
	task.ID = getNextID()
	task.CreatedAt = now
	task.LastUpdated = now

	tasks = append(tasks, task)
	saveTasksToFile()

	setFlash(w, "success", "Task creata con successo")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// UPDATE TASK FROM FORM
func updateTaskWeb(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metodo non supportato", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimSuffix(r.URL.Path, "/edit")
	id, err := extractIDFromPath(path)
	if err != nil {
		setFlash(w, "danger", "ID non valido")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	err = r.ParseForm()
	if err != nil {
		setFlash(w, "danger", "Errore lettura form")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	description := strings.TrimSpace(r.FormValue("description"))

	if title == "" {
		setFlash(w, "danger", "Il titolo è obbligatorio")
		http.Redirect(w, r, fmt.Sprintf("/tasks/%d/edit", id), http.StatusSeeOther)
		return
	}

	for i := range tasks {
		if tasks[i].ID == id {
			tasks[i].Title = title
			tasks[i].Description = description
			tasks[i].LastUpdated = time.Now()

			saveTasksToFile()
			setFlash(w, "success", "Task aggiornata con successo")
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}

	setFlash(w, "danger", "Task non trovata")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// DELETE TASK FROM WEB
func deleteTaskWeb(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/delete")
	id, err := extractIDFromPath(path)
	if err != nil {
		setFlash(w, "danger", "ID non valido")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	for i, task := range tasks {
		if task.ID == id {
			tasks = append(tasks[:i], tasks[i+1:]...)
			saveTasksToFile()
			setFlash(w, "success", "Task eliminata con successo")
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}

	setFlash(w, "danger", "Task non trovata")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// API: GET /tasks
func getTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

// API: GET /tasks/{id}
func getTaskByID(w http.ResponseWriter, r *http.Request) {
	id, err := extractIDFromPath(r.URL.Path)
	if err != nil {
		http.Error(w, "ID non valido", http.StatusBadRequest)
		return
	}

	for _, task := range tasks {
		if task.ID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(task)
			return
		}
	}

	http.Error(w, "Task non trovata", http.StatusNotFound)
}

// API: DELETE /tasks/{id}
func deleteTask(w http.ResponseWriter, r *http.Request) {
	id, err := extractIDFromPath(r.URL.Path)
	if err != nil {
		http.Error(w, "ID non valido", http.StatusBadRequest)
		return
	}

	for i, task := range tasks {
		if task.ID == id {
			tasks = append(tasks[:i], tasks[i+1:]...)
			saveTasksToFile()
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	http.Error(w, "Task non trovata", http.StatusNotFound)
}

// API: PUT /tasks/{id}
func updateTask(w http.ResponseWriter, r *http.Request) {
	id, err := extractIDFromPath(r.URL.Path)
	if err != nil {
		http.Error(w, "ID non valido", http.StatusBadRequest)
		return
	}

	var updatedTask Task
	err = json.NewDecoder(r.Body).Decode(&updatedTask)
	if err != nil {
		http.Error(w, "JSON non valido", http.StatusBadRequest)
		return
	}

	err = validateTask(updatedTask)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for i := range tasks {
		if tasks[i].ID == id {
			tasks[i].Title = updatedTask.Title
			tasks[i].Description = updatedTask.Description
			tasks[i].Done = updatedTask.Done
			tasks[i].LastUpdated = time.Now()

			saveTasksToFile()

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(tasks[i])
			return
		}
	}

	http.Error(w, "Task non trovata", http.StatusNotFound)
}

// COMPLETE TASK FROM WEB
func completeTaskWeb(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/complete")
	id, err := extractIDFromPath(path)
	if err != nil {
		setFlash(w, "danger", "ID non valido")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	for i := range tasks {
		if tasks[i].ID == id {
			tasks[i].Done = true
			tasks[i].LastUpdated = time.Now()

			saveTasksToFile()
			setFlash(w, "success", "Task completata con successo")
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}

	setFlash(w, "danger", "Task non trovata")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// VALIDATION
func validateTask(task Task) error {
	if strings.TrimSpace(task.Title) == "" {
		return fmt.Errorf("il titolo è obbligatorio")
	}
	return nil
}

// SAVE TASKS TO FILE
func saveTasksToFile() {
	file, err := os.Create(dataFile)
	if err != nil {
		fmt.Println("Errore salvataggio:", err)
		return
	}
	defer file.Close()

	err = json.NewEncoder(file).Encode(tasks)
	if err != nil {
		fmt.Println("Errore scrittura JSON:", err)
	}
}

// LOAD TASKS FROM FILE
func loadTasksFromFile() {
	file, err := os.Open(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			tasks = []Task{}
			return
		}
		fmt.Println("Errore apertura file:", err)
		return
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&tasks)
	if err != nil {
		fmt.Println("Errore lettura JSON:", err)
		tasks = []Task{}
	}
}

// NEXT ID
func getNextID() int {
	maxID := 0
	for _, task := range tasks {
		if task.ID > maxID {
			maxID = task.ID
		}
	}
	return maxID + 1
}

// EXTRACT ID FROM /tasks/3
func extractIDFromPath(path string) (int, error) {
	parts := strings.Split(path, "/")

	if len(parts) != 3 || parts[2] == "" {
		return 0, fmt.Errorf("percorso non valido")
	}

	id, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, err
	}

	return id, nil
}

func main() {
	loadTasksFromFile()

	http.HandleFunc("/", home)
	http.HandleFunc("/tasks/new", addTask)

	http.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getTasks(w, r)
		case http.MethodPost:
			createTask(w, r)
		default:
			http.Error(w, "Metodo non supportato", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/edit") {
			switch r.Method {
			case http.MethodGet:
				editTaskPage(w, r)
			case http.MethodPost:
				updateTaskWeb(w, r)
			default:
				http.Error(w, "Metodo non supportato", http.StatusMethodNotAllowed)
			}
			return
		}

		if strings.HasSuffix(r.URL.Path, "/delete") {
			deleteTaskWeb(w, r)
			return
		}

		if strings.HasSuffix(r.URL.Path, "/complete") {
			completeTaskWeb(w, r)
			return
		}

		switch r.Method {
		case http.MethodGet:
			getTaskByID(w, r)
		case http.MethodDelete:
			deleteTask(w, r)
		case http.MethodPut:
			updateTask(w, r)
		default:
			http.Error(w, "Metodo non supportato", http.StatusMethodNotAllowed)
		}
	})

	fmt.Println("Server avviato su http://localhost:8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Errore server:", err)
	}
}
