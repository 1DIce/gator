package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/1DIce/gator/internal/config"
	"github.com/1DIce/gator/internal/database"
	"github.com/1DIce/gator/internal/rss"
	"github.com/google/uuid"

	// Importing postgresql driver. It is a dependency of sqlc
	pg "github.com/lib/pq"
)

type State struct {
	config *config.Config
	db     *database.Queries
}

type cliCommand struct {
	description string
	callback    func(state *State, arguments []string) error
}

func middlewareLoggedIn(handler func(state *State, arguments []string, user database.User) error) func(*State, []string) error {
	return func(state *State, arguments []string) error {
		user, err := state.db.GetUser(context.Background(), state.config.CurrentUserName)
		if err != nil {
			return fmt.Errorf("User with name '%s' does not exist", state.config.CurrentUserName)
		}
		return handler(state, arguments, user)
	}
}

func loginCommand(state *State, arguments []string) error {
	if len(arguments) == 0 || arguments[0] == "" {
		return fmt.Errorf("User name input is missing")
	}
	if len(arguments) > 1 {
		return fmt.Errorf("Too many arguments! 'login' command expects a single argument")
	}
	userName := arguments[0]
	if _, err := state.db.GetUser(context.Background(), userName); err != nil {
		return fmt.Errorf("User with name '%s' does not exist", userName)
	}

	state.config.CurrentUserName = userName
	config.Write(*state.config)
	fmt.Printf("User has been successfully set to '%s'\n", userName)
	return nil
}

func registerUserCommand(state *State, arguments []string) error {
	if len(arguments) == 0 || arguments[0] == "" {
		return fmt.Errorf("User name input is missing")
	}
	if len(arguments) > 1 {
		return fmt.Errorf("Too many arguments! 'register' command expects a single argument")
	}

	username := arguments[0]

	now := time.Now()
	user, err := state.db.CreateUser(context.Background(), database.CreateUserParams{
		Name:      username,
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("The user does already exist")
	}

	state.config.CurrentUserName = user.Name
	config.Write(*state.config)
	fmt.Printf("User '%s' was created\n", user.Name)
	return nil
}

func listUsersCommand(state *State, arguments []string) error {
	users, err := state.db.GetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to fetch the list of users")
	}

	for _, user := range users {
		if user.Name == state.config.CurrentUserName {
			fmt.Printf("* %s (current)\n", user.Name)
		} else {
			fmt.Printf("* %s\n", user.Name)
		}
	}
	return nil
}

func resetUsersCommand(state *State, arguments []string) error {
	err := state.db.DeleteAllUsers(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to delete users with error: %v", err)
	}
	return nil
}

func aggregateFeedsCommand(state *State, arguments []string) error {
	if len(arguments) == 0 || arguments[0] == "" {
		return fmt.Errorf("Timer input is missing")
	}
	if len(arguments) > 1 {
		return fmt.Errorf("Too many arguments! 'agg' command expects a single argument")
	}

	timeBetweenRequests, err := time.ParseDuration(arguments[0])
	if err != nil {
		return fmt.Errorf("timer format is invalid: %w", err)
	}
	fmt.Printf("Collecting feeds every %s\n\n", timeBetweenRequests.String())

	ticker := time.NewTicker(timeBetweenRequests)
	for ; ; <-ticker.C {
		scrapeFeeds(state)
		fmt.Println("")
		fmt.Printf("Waiting %s to fetch the next...\n\n", timeBetweenRequests.String())
	}
}

func scrapeFeeds(state *State) error {
	feed, err := state.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to get next feed to fetch: %w", err)
	}

	feedResponse, err := rss.FetchFeed(context.Background(), feed.Url)
	if err != nil {
		return fmt.Errorf("Failed to fetch feed: %w", err)
	}

	fmt.Printf("Feed items from '%s':\n", feed.Url)
	for _, feedItem := range feedResponse.Channel.Item {
		fmt.Printf("%s\n", feedItem.Title)

		publishedAt, parseErr := parseRssPublicationDate(feedItem.PubDate)
		if parseErr != nil {
			return fmt.Errorf("Failed to parse publication date: %w", parseErr)
		}

		post, err := state.db.CreatePost(context.Background(), database.CreatePostParams{
			ID:        uuid.New(),
			Url:       feedItem.Link,
			Title:     feedItem.Title,
			CreatedAt: time.Now(),
			Description: sql.NullString{
				String: feedItem.Description,
				Valid:  true,
			},
			PublishedAt: sql.NullTime{
				Time:  publishedAt,
				Valid: true,
			},
			FeedID: feed.ID,
		})
		if err != nil {
			var postgresErr *pg.Error
			// If the post already exists we are just ignoring the error. 23505 is a duplicate key error
			if errors.As(err, &postgresErr) && postgresErr.Code != "23505" {
				return fmt.Errorf("Unexpected error occurred during post creation: %w", err)
			}
			continue
		}

		fmt.Printf("Added post '%s'\n", post.Title)
	}

	if _, err := state.db.MarkFeedFetched(context.Background(), database.MarkFeedFetchedParams{
		ID:            feed.ID,
		LastFetchedAt: sql.NullTime{Time: time.Now(), Valid: true},
	}); err != nil {
		return fmt.Errorf("Failed to update last fetched timestamp: %v", err)
	}

	return nil
}

func parseRssPublicationDate(rssPublicationDate string) (time.Time, error) {
	// the input date comes in this format: Sun, 03 Dec 2023 00:00:00 +0000
	timeLayout := "2006-Jan-02"
	splits := strings.Split(rssPublicationDate, " ")
	relevantSplits := splits[1:4]
	slices.Reverse(relevantSplits)
	joined := strings.Join(relevantSplits, "-")
	fmt.Println(joined)
	return time.Parse(timeLayout, joined)
}

func addFeedCommand(state *State, arguments []string, user database.User) error {
	if len(arguments) == 0 || arguments[0] == "" {
		return fmt.Errorf("Feed url input is missing")
	}
	if len(arguments) > 2 {
		return fmt.Errorf("Too many arguments! 'addfeed' command expects a 2 arguments: name and feed url")
	}

	feedUrl := arguments[1]
	feedName := arguments[0]

	// We want to make sure the the url points to a valid feed
	if _, err := rss.FetchFeed(context.Background(), feedUrl); err != nil {
		return fmt.Errorf("Failed to fetch feed with error: %v", err)
	}

	now := time.Now()
	feed, err := state.db.CreateFeed(context.Background(), database.CreateFeedParams{
		ID:        uuid.New(),
		Name:      feedName,
		Url:       feedUrl,
		CreatedAt: now,
		UpdatedAt: now,
		UserID:    user.ID,
	})
	if err != nil {
		return fmt.Errorf("Failed to add feed for url '%s' with error: %v", feedUrl, err)
	}
	fmt.Printf("Successfully added feed with name '%s' and url '%s'\n", feedName, feedUrl)

	state.db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID:        uuid.New(),
		FeedID:    feed.ID,
		UserID:    user.ID,
		CreatedAt: now,
		UpdatedAt: now,
	})
	fmt.Printf("You are now following '%s'\n", feedName)

	return nil
}

func listFeedsCommand(state *State, arguments []string) error {
	feeds, err := state.db.ListFeeds(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to fetch feeds with error: %v", err)
	}

	fmt.Println("Name\tUrl\tUser")
	for _, feed := range feeds {
		fmt.Printf("%s\t%s\t%s\n", feed.Name, feed.Url, feed.UserName)
	}
	return nil
}

func followFeedCommand(state *State, arguments []string, user database.User) error {
	if len(arguments) == 0 || arguments[0] == "" {
		return fmt.Errorf("Feed url input is missing")
	}
	if len(arguments) > 1 {
		return fmt.Errorf("Too many arguments! 'follow' command expects a single argument")
	}

	feedUrl := arguments[0]
	feed, err := state.db.GetFeed(context.Background(), feedUrl)
	if err != nil {
		return fmt.Errorf("Failed to find feed by url: %w", err)
	}

	now := time.Now()
	feedFollow, err := state.db.CreateFeedFollow(context.Background(),
		database.CreateFeedFollowParams{
			ID:        uuid.New(),
			FeedID:    feed.ID,
			UserID:    user.ID,
			CreatedAt: now,
			UpdatedAt: now,
		})
	if err != nil {
		return fmt.Errorf("Failed to created follow: %w", err)
	}

	fmt.Printf("User '%s' is now following the feed '%s'\n", feedFollow.UserName, feedFollow.FeedName)

	return nil
}

func listFollowedFeedsCommand(state *State, arguments []string, user database.User) error {
	follows, err := state.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return fmt.Errorf("Failed to fetch followed feeds: %w", err)
	}

	for _, follow := range follows {
		fmt.Printf("%s\n", follow.FeedName)
	}
	return nil
}

func unfollowFeedCommand(state *State, arguments []string, user database.User) error {
	if len(arguments) == 0 || arguments[0] == "" {
		return fmt.Errorf("Feed url input is missing")
	}
	if len(arguments) > 1 {
		return fmt.Errorf("Too many arguments! 'unfollow' command expects a single argument")
	}

	feedUrl := arguments[0]
	if _, err := state.db.DeleteFeedFollow(context.Background(), database.DeleteFeedFollowParams{
		UserID:  user.ID,
		FeedUrl: feedUrl,
	}); err != nil {
		return fmt.Errorf("Failed to delete follow", err)
	}

	fmt.Printf("Successfully unfollowed feed\n")

	return nil
}

func browsePostsCommand(state *State, arguments []string, user database.User) error {

	limit := 2
	if len(arguments) > 0 {
		parsedLimit, err := strconv.Atoi(arguments[0])
		if err != nil {
			return fmt.Errorf("The limit input is not a valid integer: %w", err)
		}
		limit = int(parsedLimit)
	}

	posts, err := state.db.GetPostsForUser(context.Background(), database.GetPostsForUserParams{
		UserID: user.ID,
		Limit:  int32(limit),
	})
	if err != nil {
		return fmt.Errorf("Failed to retrieve posts: %w", err)
	}

	for _, post := range posts {
		fmt.Printf("%s\t%s\n", post.Title, post.Url)
	}

	return nil
}

func getCliCommands() map[string]cliCommand {
	return map[string]cliCommand{
		"login": {
			description: "Sets the current user in the config",
			callback:    loginCommand,
		},
		"register": {
			description: "Registers a new user",
			callback:    registerUserCommand,
		},
		"users": {
			description: "Lists all registered users",
			callback:    listUsersCommand,
		},
		"reset": {
			description: "Delete all users",
			callback:    resetUsersCommand,
		},
		"agg": {
			description: "start long running aggregator service",
			callback:    aggregateFeedsCommand,
		},
		"addfeed": {
			description: "add a new RSS feed url",
			callback:    middlewareLoggedIn(addFeedCommand),
		},
		"feeds": {
			description: "list all stored RSS feeds",
			callback:    listFeedsCommand,
		},
		"follow": {
			description: "Follow a registered feed by url",
			callback:    middlewareLoggedIn(followFeedCommand),
		},
		"following": {
			description: "Lists all feeds the user is following",
			callback:    middlewareLoggedIn(listFollowedFeedsCommand),
		},
		"unfollow": {
			description: "Unfollows a given feed url",
			callback:    middlewareLoggedIn(unfollowFeedCommand),
		},
		"browse": {
			description: "List saved posts with an optional limit",
			callback:    middlewareLoggedIn(browsePostsCommand),
		},
	}
}

func main() {
	availableCommands := getCliCommands()
	config := loadConfig()
	db, err := sql.Open("postgres", config.DbURL)
	if err != nil {
		log.Fatalf("Failed to open database connection with '%s'", config.DbURL)
	}

	dbQueries := database.New(db)

	state := State{config: &config, db: dbQueries}

	arguments := os.Args
	if len(arguments) < 2 {
		log.Fatalf("No command given. See 'help' for a list of available commands")
	}
	command, ok := availableCommands[arguments[1]]
	if !ok {
		log.Fatalf("'%s' is not a valid command!\n", arguments)
	}

	if err := command.callback(&state, arguments[2:]); err != nil {
		log.Fatalf("Error during command execution: %v\n", err)
	}
}

func loadConfig() config.Config {
	configFile, err := config.Read()
	if err != nil {
		log.Fatal("No config file was found")
	}

	return configFile
}
