package main

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Product structure represents a product in the store
type Product struct {
	ID    primitive.ObjectID `bson:"_id"`
	Name  string             `bson:"name"`
	Size  string             `bson:"size" json:"size"`
	Price int                `bson:"price" json:"price"`
}

// User structure represents a user in the system
type User struct {
	Username string
	Email    string
	Password string
	Role     string
}

var (
	db        *mongo.Database // Adjusted for MongoDB
	templates = template.Must(template.ParseGlob("templates/*.html"))
)

func initDB() {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb+srv://erkezhan_a_e:AE070120@cluster0.ca1wa.mongodb.net/"))
	if err != nil {
		fmt.Println("Error creating MongoDB client:", err)
		os.Exit(1)
	}

	err = client.Ping(context.TODO(), readpref.Primary())
	if err != nil {
		fmt.Println("Error connecting to MongoDB:", err)
		os.Exit(1)
	}

	db = client.Database("usersAuth")
	fmt.Println("Connected to MongoDB")
}

func fetchProductsFromDB() ([]Product, error) {
	var products []Product

	collection := db.Collection("laptops")
	cur, err := collection.Find(context.TODO(), bson.M{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(context.TODO())

	for cur.Next(context.TODO()) {
		var p Product
		err := cur.Decode(&p)
		if err != nil {
			continue
		}
		products = append(products, p)
	}

	if err := cur.Err(); err != nil {
		return nil, err
	}

	return products, nil
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authentication
		cookie, err := r.Cookie("username")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		username := cookie.Value

		collection := db.Collection("users")

		var user User
		// Use '=' instead of ':=' because 'err' is already declared
		err = collection.FindOne(context.TODO(), bson.M{
			"username": username,
		}).Decode(&user)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if user.Role != "admin" {
			http.Error(w, "You're not an admin", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func IsLoggedIn(r *http.Request) bool {
	cookie, err := r.Cookie("username")
	if err == nil && cookie != nil && cookie.Value != "" {
		return true
	}
	return false
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := templates.Lookup("register.html")
	if tmpl == nil {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

func RegisterPostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	username := r.FormValue("username")
	email := r.FormValue("email")
	password := r.FormValue("password")
	role := "user"

	if username == "Yerkezhan" || username == "admin" {
		role = "admin"
	}

	collection := db.Collection("users")
	_, err := collection.InsertOne(context.TODO(), bson.M{
		"username": username,
		"email":    email,
		"password": password,
		"role":     role,
	})

	if err != nil {
		http.Error(w, "Registration failed", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "User %s successfully registered", username)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := templates.Lookup("login.html")
	if tmpl == nil {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

func LoginPostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	var user User
	collection := db.Collection("users")
	err := collection.FindOne(context.TODO(), bson.M{
		"username": username,
		"password": password,
	}).Decode(&user)

	if err != nil {
		http.Error(w, "Login failed", http.StatusUnauthorized)
		return
	}

	expiration := time.Now().Add(24 * time.Hour)
	cookie := http.Cookie{Name: "username", Value: username, Expires: expiration}
	http.SetCookie(w, &cookie)

	if user.Role == "admin" {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/profile-edit", http.StatusSeeOther)
	}
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie := http.Cookie{
		Name:    "username",
		Value:   "",
		Expires: time.Now().Add(-time.Hour),
	}
	http.SetCookie(w, &cookie)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {

	isLoggedIn := IsLoggedIn(r)

	// Fetch products from the database
	products, err := fetchProductsFromDB()
	if err != nil {
		http.Error(w, "Error fetching products from the database", http.StatusInternalServerError)
		return
	}

	// Prepare data for the template
	tmpl := templates.Lookup("index.html")
	if tmpl == nil {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	data := struct {
		Products   []Product
		IsLoggedIn bool
	}{
		Products:   products,
		IsLoggedIn: isLoggedIn,
	}

	// Render the template with the data
	tmpl.Execute(w, data)
}

func AdminHandler(w http.ResponseWriter, r *http.Request) {
	// Fetch products from the database
	products, err := fetchProductsFromDB()
	if err != nil {
		http.Error(w, "Error fetching products from the database", http.StatusInternalServerError)
		return
	}

	tmpl := templates.Lookup("admin.html")
	if tmpl == nil {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	data := struct {
		Products []Product
	}{
		Products: products,
	}

	tmpl.Execute(w, data)
}

func ProfileEditHandler(w http.ResponseWriter, r *http.Request) {
	// Fetch user profile information from the database based on the logged-in user
	cookie, err := r.Cookie("username")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	username := cookie.Value

	var user User
	err = db.Collection("users").FindOne(context.TODO(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		http.Error(w, "Error fetching user profile from the database", http.StatusInternalServerError)
		return
	}

	// Parse the HTML template file
	tmpl := templates.Lookup("profile-edit.html")
	if tmpl == nil {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	// Execute the template with user profile data
	tmpl.Execute(w, user)
}

func ProfileEditPostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	// Fetch user profile information from the form submission
	username := r.FormValue("username")
	email := r.FormValue("email")
	password := r.FormValue("password")

	update := bson.M{"$set": bson.M{"email": email}}
	if password != "" {
		update["$set"].(bson.M)["password"] = password
	}

	_, err := db.Collection("users").UpdateOne(
		context.TODO(),
		bson.M{"username": username},
		update,
	)
	if err != nil {
		log.Println("Error updating user profile in MongoDB:", err)
		http.Error(w, "Error updating user profile in database", http.StatusInternalServerError)
		return
	}

	// Redirect to the profile page or any other page after successful update
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func DeleteHandler(w http.ResponseWriter, r *http.Request) {
	productID := r.URL.Path[len("/delete/"):]
	if productID == "" {
		http.Error(w, "Product ID not provided", http.StatusBadRequest)
		return
	}

	collection := db.Collection("laptops")
	objectID, err := primitive.ObjectIDFromHex(productID)
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	_, err = collection.DeleteOne(context.TODO(), bson.M{"_id": objectID})
	if err != nil {
		http.Error(w, "Error deleting product", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func AddProductHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := templates.Lookup("add-product.html")
	if tmpl == nil {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

func AddProductPostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	name := r.FormValue("name")
	size := r.FormValue("size")
	price, _ := strconv.Atoi(r.FormValue("price")) // Convert price to int

	_, err := db.Collection("laptops").InsertOne(context.TODO(), bson.M{
		"name":  name,
		"size":  size,
		"price": price,
	})
	if err != nil {
		log.Println("Error inserting into MongoDB:", err)
		http.Error(w, "Error inserting into database", http.StatusInternalServerError)
		return
	}

	log.Printf("New product added: Name=%s, Size=%s, Price=%d\n", name, size, price)

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func EditProductHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the hex ID from the URL, ensuring it's just the hex string without any encoding.
	hexID := strings.TrimPrefix(r.URL.Path, "/edit/")

	// Log the hexID for debugging purposes
	log.Printf("Hex ID received: %s", hexID)

	// Convert the hex string to an ObjectID.
	objectID, err := primitive.ObjectIDFromHex(hexID)
	if err != nil {
		log.Printf("Error converting hex to ObjectID: %v, hex: %s", err, hexID)
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	// Log the objectID for debugging purposes
	log.Printf("ObjectID to query: %s", objectID.Hex())

	var product Product
	err = db.Collection("laptops").FindOne(context.TODO(), bson.M{"_id": objectID}).Decode(&product)
	if err != nil {
		log.Printf("Error fetching product details from MongoDB: %v, ObjectID: %s", err, objectID.Hex())
		http.Error(w, "Error fetching product details", http.StatusInternalServerError)
		return
	}

	tmpl := templates.Lookup("edit-product.html")
	if tmpl == nil {
		log.Println("Template not found")
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, product); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
	}
}

func EditProductPostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Path[len("/edit-product-post/"):]
	// Convert the hex string to an ObjectID
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	// Assuming the price is sent as an integer value in cents (or the smallest currency unit)
	price, err := strconv.Atoi(r.FormValue("price"))
	if err != nil {
		http.Error(w, "Invalid price", http.StatusBadRequest)
		return
	}

	_, err = db.Collection("laptops").UpdateOne(
		context.TODO(),
		bson.M{"_id": objectID}, // Use the ObjectID for the update filter
		bson.M{
			"$set": bson.M{
				"name":  r.FormValue("name"),
				"size":  r.FormValue("size"),
				"price": price,
			},
		},
	)
	if err != nil {
		log.Println("Error updating product in MongoDB:", err)
		http.Error(w, "Error updating product in database", http.StatusInternalServerError)
		return
	}

	log.Printf("Product updated with ID: %s\n", objectID.Hex())

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func main() {

	// Initialize MongoDB client
	initDB() // Adjusted to not assign to db since initDB() now sets the global variable directly

	// Set up routes
	http.HandleFunc("/register", RegisterHandler)
	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/register-post", RegisterPostHandler)
	http.HandleFunc("/login-post", LoginPostHandler)
	http.HandleFunc("/logout", LogoutHandler)
	http.HandleFunc("/", IndexHandler)
	http.Handle("/admin", AuthMiddleware(http.HandlerFunc(AdminHandler)))
	http.HandleFunc("/profile-edit", ProfileEditHandler)
	http.HandleFunc("/profile-edit-post", ProfileEditPostHandler)
	http.HandleFunc("/delete/", DeleteHandler)
	http.HandleFunc("/add-product", AddProductHandler)
	http.HandleFunc("/add-product-post", AddProductPostHandler)
	http.HandleFunc("/edit/", EditProductHandler)
	http.HandleFunc("/edit-product-post/", EditProductPostHandler)

	http.ListenAndServe(":8080", nil)

}
