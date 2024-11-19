package main

import (
	"encoding/csv"
	//"encoding/json"
	"fmt"
	//"io"
	"log"
	"math"
	//"net/http"
	"os"
	"sort"
	"strconv"
	//"strings"
)

const APIKey = "api_key_hihi"

type MovieRating struct {
	PersonID   int
	MovieTitle string
	IMDBID     string
	Rating     float64
}

type MovieRatings struct {
	Ratings []MovieRating
}

func (mr *MovieRatings) LoadCSV(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("could not open file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("could not read CSV: %v", err)
	}

	for i, record := range records {
		if i == 0 {
			continue
		}

		if len(record) < 3 {
			log.Printf("Invalid record at line %d: expected at least 3 fields, got %d", i+1, len(record))
			continue
		}

		personID, err := strconv.Atoi(record[0])
		if err != nil {
			log.Printf("Invalid PersonID at line %d: %v", i+1, err)
			continue
		}

		rating := parseRating(record[2])
		mr.Ratings = append(mr.Ratings, MovieRating{
			PersonID:   personID,
			MovieTitle: record[1],
			Rating:     rating,
		})
	}

	return nil
}

func (mr *MovieRatings) LoadIMDBIDs(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("could not open file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("could not read CSV: %v", err)
	}

	imdbMap := make(map[string]string)
	for i, record := range records {
		if i == 0 {
			continue
		}
		if len(record) < 2 {
			log.Printf("Invalid record at line %d: expected at least 2 fields, got %d", i+1, len(record))
			continue
		}
		imdbMap[record[0]] = record[1]
	}

	for i, rating := range mr.Ratings {
		if imdbID, ok := imdbMap[rating.MovieTitle]; ok {
			mr.Ratings[i].IMDBID = imdbID
		}
	}
	return nil
}

func (mr *MovieRatings) RecommendMovies(personID, k int) ([]string, []string) {
	userRatings := mr.groupRatingsByPerson()
	users := make([]int, 0, len(userRatings))
	for user := range userRatings {
		users = append(users, user)
	}

	centroids := initializeCentroids(users, k)
	clusters := make(map[int][]int)

	for i := 0; i < 10; i++ {
		clusters = assignToClusters(users, centroids, userRatings)
		centroids = updateCentroids(clusters, userRatings)
	}

	userCluster := findCluster(personID, clusters)
	if userCluster == -1 {
		return nil, nil
	}

	recommendations := calculateRecommendations(personID, userRatings, clusters[userCluster])
	return extractTopRecommendations(recommendations, 5), extractBottomRecommendations(recommendations, 5)
}

func (mr *MovieRatings) groupRatingsByPerson() map[int]map[string]float64 {
	userRatings := make(map[int]map[string]float64)
	for _, rating := range mr.Ratings {
		if _, exists := userRatings[rating.PersonID]; !exists {
			userRatings[rating.PersonID] = make(map[string]float64)
		}
		userRatings[rating.PersonID][rating.MovieTitle] = rating.Rating
	}
	return userRatings
}

func calculateDistance(user1, user2 map[string]float64) float64 {
	sum := 0.0
	for movie, rating1 := range user1 {
		rating2, exists := user2[movie]
		if exists {
			sum += (rating1 - rating2) * (rating1 - rating2)
		} else {
			sum += rating1 * rating1
		}
	}
	for movie, rating2 := range user2 {
		if _, exists := user1[movie]; !exists {
			sum += rating2 * rating2
		}
	}
	return math.Sqrt(sum)
}

func parseRating(ratingStr string) float64 {
	rating, err := strconv.ParseFloat(ratingStr, 64)
	if err != nil {
		return 0.0
	}
	return rating
}

func initializeCentroids(users []int, k int) []int {
	centroids := make([]int, k)
	for i := 0; i < k; i++ {
		centroids[i] = users[i%len(users)]
	}
	return centroids
}

func assignToClusters(users, centroids []int, userRatings map[int]map[string]float64) map[int][]int {
	clusters := make(map[int][]int)
	for _, user := range users {
		closestCentroid := -1
		minDistance := math.MaxFloat64
		for i, centroid := range centroids {
			distance := calculateDistance(userRatings[user], userRatings[centroid])
			if distance < minDistance {
				closestCentroid = i
				minDistance = distance
			}
		}
		clusters[closestCentroid] = append(clusters[closestCentroid], user)
	}
	return clusters
}

func updateCentroids(clusters map[int][]int, userRatings map[int]map[string]float64) []int {
	newCentroids := make([]int, len(clusters))
	for i, cluster := range clusters {
		if len(cluster) == 0 {
			newCentroids[i] = -1 // Placeholder for empty cluster
		} else {
			newCentroids[i] = cluster[0] // Simple replacement, can calculate mean user profile here
		}
	}
	return newCentroids
}

func findCluster(personID int, clusters map[int][]int) int {
	for clusterID, users := range clusters {
		for _, user := range users {
			if user == personID {
				return clusterID
			}
		}
	}
	return -1
}

func calculateRecommendations(personID int, userRatings map[int]map[string]float64, clusterUsers []int) map[string]float64 {
	seenMovies := userRatings[personID]
	recommendations := make(map[string]float64)
	for _, user := range clusterUsers {
		for movie, rating := range userRatings[user] {
			if _, seen := seenMovies[movie]; !seen {
				recommendations[movie] += rating
			}
		}
	}
	return recommendations
}

func extractTopRecommendations(recommendations map[string]float64, count int) []string {
	type movieRecommendation struct {
		Movie  string
		Rating float64
	}
	movieList := make([]movieRecommendation, 0, len(recommendations))
	for movie, rating := range recommendations {
		movieList = append(movieList, movieRecommendation{Movie: movie, Rating: rating})
	}
	sort.Slice(movieList, func(i, j int) bool { return movieList[i].Rating > movieList[j].Rating })

	topMovies := make([]string, 0, count)
	for i := 0; i < count && i < len(movieList); i++ {
		topMovies = append(topMovies, movieList[i].Movie)
	}
	return topMovies
}

func extractBottomRecommendations(recommendations map[string]float64, count int) []string {
	type movieRecommendation struct {
		Movie  string
		Rating float64
	}
	movieList := make([]movieRecommendation, 0, len(recommendations))
	for movie, rating := range recommendations {
		movieList = append(movieList, movieRecommendation{Movie: movie, Rating: rating})
	}
	sort.Slice(movieList, func(i, j int) bool { return movieList[i].Rating < movieList[j].Rating })

	bottomMovies := make([]string, 0, count)
	for i := 0; i < count && i < len(movieList); i++ {
		bottomMovies = append(bottomMovies, movieList[i].Movie)
	}
	return bottomMovies
}

func main() {
	mr := &MovieRatings{}

	if err := mr.LoadCSV("dane.csv"); err != nil {
		log.Fatalf("Error loading movie ratings: %v", err)
	}

	if err := mr.LoadIMDBIDs("imdb.csv"); err != nil {
		log.Fatalf("Error loading IMDB IDs: %v", err)
	}

	topMovies, bottomMovies := mr.RecommendMovies(1, 2)

	fmt.Println("Top 5 recommended movies:")
	for _, movie := range topMovies {
		fmt.Println(movie)
	}

	fmt.Println("\nTop 5 least recommended movies:")
	for _, movie := range bottomMovies {
		fmt.Println(movie)
	}
}
