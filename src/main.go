package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/1DIce/gator/internal/config"
	"github.com/1DIce/gator/internal/database"
	"github.com/1DIce/gator/internal/rss"
	"github.com/google/uuid"

	// Importing postgresql driver. It is a dependency of sqlc
	_ "github.com/lib/pq"
)

type State struct {
	config *config.Config
	db     *database.Queries
}

type cliCommand struct {
	description string
	callback    func(state *State, arguments []string) error
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

func fetchFeedCommand(state *State, arguments []string) error {
	feed, err := rss.FetchFeed(context.Background(), "https://www.wagslane.dev/index.xml")
	if err != nil {
		return err
	}
	fmt.Printf("%v\n", *feed)
	return nil
}

func addFeedCommand(state *State, arguments []string) error {
	if len(arguments) == 0 || arguments[0] == "" {
		return fmt.Errorf("Feeda url input is missing")
	}
	if len(arguments) > 2 {
		return fmt.Errorf("Too many arguments! 'addfeed' command expects a 2 arguments: name and feed url")
	}

	user, err := state.db.GetUser(context.Background(), state.config.CurrentUserName)
	if err != nil {
		return fmt.Errorf("No logged in user found")
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

func followFeedCommand(state *State, arguments []string) error {
	if len(arguments) == 0 || arguments[0] == "" {
		return fmt.Errorf("Feed url input is missing")
	}
	if len(arguments) > 1 {
		return fmt.Errorf("Too many arguments! 'follow' command expects a single argument")
	}

	user, err := state.db.GetUser(context.Background(), state.config.CurrentUserName)
	if err != nil {
		return fmt.Errorf("Failed to find user info for current user name: %w", err)
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

func listFollowedFeedsCommand(state *State, arguments []string) error {
	follows, err := state.db.GetFeedFollowsForUser(context.Background(), state.config.CurrentUserName)
	if err != nil {
		return fmt.Errorf("Failed to fetch followed feeds: %w", err)
	}

	for _, follow := range follows {
		fmt.Printf("%s\n", follow.FeedName)
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
			callback:    fetchFeedCommand,
		},
		"addfeed": {
			description: "add a new RSS feed url",
			callback:    addFeedCommand,
		},
		"feeds": {
			description: "list all stored RSS feeds",
			callback:    listFeedsCommand,
		},
		"follow": {
			description: "Follow a registered feed by url",
			callback:    followFeedCommand,
		},
		"following": {
			description: "Lists all feeds the user is following",
			callback:    listFollowedFeedsCommand,
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
