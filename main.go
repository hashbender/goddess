package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"github.com/jpmonette/force"
	"googlemaps.github.io/maps"
)

var (
	street = flag.String("street", "", "Text Search query to execute.")
	city   = flag.String("city", "", "Text Search query to execute.")
	state  = flag.String("state", "", "Text Search query to execute.")
	zip    = flag.String("zip", "", "Text Search query to execute.")
)

type Credentials struct {
	AccessToken string `json:"access_token"`
	InstanceURL string `json:"instance_url"`
	IssuedAt    int
	ID          string
	TokenType   string `json:"token_type"`
	Signature   string
}

func main() {
	flag.Parse()
	client, err := maps.NewClient(maps.WithAPIKey(PLACE_API_KEY))
	if err != nil {
		log.Fatalf("fatal error1: %s", err)
	}
	query := fmt.Sprintf("%s %s %s %s", *street, *city, *state, *zip)
	resp, err := client.TextSearch(context.Background(), &maps.TextSearchRequest{Query: query})
	if err != nil {
		log.Fatalf("fatal error2: %s", err)
	}
	placeID := resp.Results[0].PlaceID

	detailResp, err := client.PlaceDetails(context.Background(), &maps.PlaceDetailsRequest{PlaceID: placeID})
	if err != nil {
		log.Fatalf("fatal error2: %s", err)
	}

	streetNumber := ""
	street := ""
	zip := ""
	state := ""
	city := ""
	for _, comp := range detailResp.AddressComponents {
		if contains(comp.Types, "postal_code") {
			zip = comp.ShortName
		}
		if contains(comp.Types, "street_number") {
			streetNumber = comp.ShortName
		}
		if contains(comp.Types, "route") {
			street = comp.ShortName
		}
		if contains(comp.Types, "administrative_area_level_1") {
			state = comp.ShortName
		}
		if contains(comp.Types, "locality") {
			city = comp.ShortName
		}
	}
	log.Printf("StreetNumber: %s, Street: %s, City: %s, State: %s, Zip: %s", streetNumber, street, city, state, zip)

	cookieJar, _ := cookiejar.New(nil)
	log.Println("Creating cookiejar")

	forceClient := &http.Client{
		Jar: cookieJar,
	}

	authResp, err := forceClient.PostForm("https://login.salesforce.com/services/oauth2/token", params)
	if err != nil {
		log.Println("Error posting auth form")
		panic(err)
	}

	decoder := json.NewDecoder(authResp.Body)
	var creds Credentials
	if err = decoder.Decode(&creds); err == io.EOF {
		log.Fatal(err)
	} else if err != nil {
		log.Fatal(err)
	} else if creds.AccessToken == "" {
		fmt.Printf("%#v", creds)
		log.Fatalf("Unable to fetch access token. Check credentials in environmental variables")
	}
	authResp.Body.Close()

	forceApi, err := force.NewClient(forceClient, "https://na42.salesforce.com")
	if err != nil {
		panic(err)
	}
	req, err := forceApi.NewRequest("GET", "/query/?q="+url.QueryEscape("SELECT Id, Zip_Code__c, Street_Address__c, State_Code__c, Place_ID__c, City__c FROM Property__c"), nil)
	if err != nil {
		panic(err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", creds.AccessToken))
	if err != nil {
		panic(err)
	}
	property := &Property{Street: streetNumber + " " + street, State: state, City: city, Zip: zip}

	req, err = forceApi.NewRequest("PATCH", "/sobjects/Property__c/Place_ID__c/"+placeID, property)
	if err != nil {
		panic(err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", creds.AccessToken))

	var result Response
	err = forceApi.Do(req, &result)
	if err != nil {
		panic(err)
	}
	log.Printf("Result %v", result)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

type Response struct {
	Records []Property `json:"records"`
}

type Property struct {
	Zip    string `json:"Zip_Code__c"`
	Street string `json:"Street_Address__c"`
	State  string `json:"State_Code__c"`
	City   string `json:"City__c"`
}
