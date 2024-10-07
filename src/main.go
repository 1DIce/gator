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
