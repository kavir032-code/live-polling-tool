package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type authRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type pollRequest struct {
	Title   string   `json:"title"`
	Options []string `json:"options"`
}

type voteRequest struct {
	OptionID string `json:"optionId"`
	VoterKey string `json:"voterKey"`
}

func (a *App) Signup(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Name == "" || req.Email == "" || len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, valid email and password of at least 6 characters are required"})
		return
	}

	ctx := c.Request.Context()
	users := a.DB.Collection("users")

	var existing User
	err := users.FindOne(ctx, bson.M{"email": req.Email}).Decode(&existing)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create account"})
		return
	}

	user := User{
		ID:           primitive.NewObjectID(),
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: hash,
		CreatedAt:    time.Now(),
	}

	if _, err := users.InsertOne(ctx, user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create account"})
		return
	}

	token, err := makeToken(a.JWTSecret, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"token": token, "user": gin.H{
		"id": user.ID.Hex(), "name": user.Name, "email": user.Email,
	}})
}

func (a *App) Login(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	var user User
	err := a.DB.Collection("users").FindOne(c.Request.Context(), bson.M{"email": req.Email}).Decode(&user)
	if err != nil || checkPassword(user.PasswordHash, req.Password) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	token, err := makeToken(a.JWTSecret, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token, "user": gin.H{
		"id": user.ID.Hex(), "name": user.Name, "email": user.Email,
	}})
}

func (a *App) CreatePoll(c *gin.Context) {
	var req pollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if len(req.Title) < 3 || len(req.Title) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title must be between 3 and 200 characters"})
		return
	}

	if len(req.Options) < 2 || len(req.Options) > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "poll must have between 2 and 10 options"})
		return
	}

	options := make([]PollOption, 0, len(req.Options))
	seen := map[string]bool{}
	for _, raw := range req.Options {
		text := strings.TrimSpace(raw)
		if text == "" || len(text) > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "each option must contain 1 to 100 characters"})
			return
		}
		key := strings.ToLower(text)
		if seen[key] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "options must be unique"})
			return
		}
		seen[key] = true
		options = append(options, PollOption{ID: newID(), Text: text})
	}

	userID, _ := c.Get("userID")
	creatorID := userID.(primitive.ObjectID)

	poll := Poll{
		ID:        primitive.NewObjectID(),
		Title:     req.Title,
		Options:   options,
		CreatedBy: creatorID,
		ShareID:   newID(),
		CreatedAt: time.Now(),
	}

	if _, err := a.DB.Collection("polls").InsertOne(c.Request.Context(), poll); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create poll"})
		return
	}

	// Initialize Redis counts so Redis has a meaningful role in live results.
	ctx := c.Request.Context()
	for _, option := range poll.Options {
		if err := a.Redis.Set(ctx, countKey(poll.ID.Hex(), option.ID), 0, 0).Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not initialize live results"})
			return
		}
	}

	c.JSON(http.StatusCreated, poll)
}

func (a *App) GetPoll(c *gin.Context) {
	poll, err := a.findPoll(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
		return
	}
	c.JSON(http.StatusOK, poll)
}

func (a *App) GetResults(c *gin.Context) {
	poll, err := a.findPoll(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
		return
	}

	result := map[string]int64{}
	for _, option := range poll.Options {
		n, err := a.Redis.Get(c.Request.Context(), countKey(poll.ID.Hex(), option.ID)).Int64()
		if err != nil {
			n = 0
		}
		result[option.ID] = n
	}

	c.JSON(http.StatusOK, gin.H{"pollId": poll.ID.Hex(), "counts": result})
}

func (a *App) Vote(c *gin.Context) {
	var req voteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	req.OptionID = strings.TrimSpace(req.OptionID)
	req.VoterKey = strings.TrimSpace(req.VoterKey)
	if req.OptionID == "" || len(req.VoterKey) < 10 || len(req.VoterKey) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid vote data"})
		return
	}

	poll, err := a.findPoll(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
		return
	}

	validOption := false
	for _, option := range poll.Options {
		if option.ID == req.OptionID {
			validOption = true
			break
		}
	}
	if !validOption {
		c.JSON(http.StatusBadRequest, gin.H{"error": "option does not belong to this poll"})
		return
	}

	votes := a.DB.Collection("votes")
	filter := bson.M{"pollId": poll.ID, "voterKey": req.VoterKey}
	count, err := votes.CountDocuments(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not check vote"})
		return
	}
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "you have already voted"})
		return
	}

	vote := Vote{
		ID:        primitive.NewObjectID(),
		PollID:    poll.ID,
		OptionID:  req.OptionID,
		VoterKey:  req.VoterKey,
		CreatedAt: time.Now(),
	}

	if _, err := votes.InsertOne(c.Request.Context(), vote); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save vote"})
		return
	}

	// Redis is the live count source.
	newCount, err := a.Redis.Incr(c.Request.Context(), countKey(poll.ID.Hex(), req.OptionID)).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update live count"})
		return
	}

	event := map[string]interface{}{
		"pollId":   poll.ID.Hex(),
		"optionId": req.OptionID,
		"count":    newCount,
	}
	if err := a.publishUpdate(c.Request.Context(), poll.ID.Hex(), event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not publish live update"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "vote recorded", "count": newCount})
}

func (a *App) findPoll(ctx context.Context, id string) (*Poll, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var poll Poll
	if err := a.DB.Collection("polls").FindOne(ctx, bson.M{"_id": oid}).Decode(&poll); err != nil {
		return nil, err
	}
	return &poll, nil
}

func newID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return primitive.NewObjectID().Hex()
	}
	return hex.EncodeToString(b)
}

func countKey(pollID, optionID string) string {
	return "poll:" + pollID + ":option:" + optionID + ":count"
}

func (a *App) publishUpdate(ctx context.Context, pollID string, event interface{}) error {
	data, err := bson.MarshalExtJSON(event, false, false)
	if err != nil {
		return err
	}
	return a.Redis.Publish(ctx, "poll:"+pollID, string(data)).Err()
}
