package rcommons

import (
	"log"
	"sync"

	"github.com/bancolombia/reactive-commons-go/internal/rabbit"
	"github.com/google/uuid"
)

const domainEvents = "domainEvents"
const domainEventsSuffix = "subsEvents"

const directExchange = "directMessages"
const directCommandsSuffix = "direct"

const globalExchange = "globalReply"
const directQuerySuffix = "query"
const globalRepliesSuffix = "replies"

const ChannelForEvents string = "ChannelForEvents"
const ChannelForCommands string = "ChannelForCommands"
const ChannelForQueries string = "ChannelForQueries"
const ChannelForReplies string = "ChannelForReplies"

type ReactiveCommons struct {
	Domain          DomainDefinition
	rclient         *rabbit.RabbitClient
	registry        *Registry
	eventPublisher  *RabbitEventPublisher
	eventListener   *RabbitEventListener
	commandSender   *RabbitCommandSender
	commandReceiver *RabbitCommandReceiver
	queryClient     *RabbitQueryClient
	queryServer     *RabbitQueryServer
	topologyManager *RabbitTopologyManager
}

func NewReactiveCommons(domainCfg DomainDefinition) *ReactiveCommons {
	if domainCfg.Name == "" {
		log.Panic("invalid domain name")
	}
	connAddr := connectionString(domainCfg.ConnectionConfig)
	cli, _ := rabbit.NewRabbitClient(domainCfg.Name, connAddr)
	err := cli.Connect()
	if err != nil {
		log.Panicf("Failed to connect to RabbitMQ: %v", err)
	}

	registry := &Registry{}

	// Setup default names for events, if not provided
	if domainCfg.DomainEventsExchange == "" {
		domainCfg.DomainEventsExchange = domainEvents
	}
	if domainCfg.DomainEventsSuffix == "" {
		domainCfg.DomainEventsSuffix = domainEventsSuffix
	}
	// for commands
	if domainCfg.DirectExchange == "" {
		domainCfg.DirectExchange = directExchange
	}
	if domainCfg.DirectCommandsSuffix == "" {
		domainCfg.DirectCommandsSuffix = directCommandsSuffix
	}
	// for queries
	if domainCfg.DirectQuerySuffix == "" {
		domainCfg.DirectQuerySuffix = directQuerySuffix
	}
	if domainCfg.GlobalExchange == "" {
		domainCfg.GlobalExchange = globalExchange
	}
	if domainCfg.GlobalRepliesSuffix == "" {
		domainCfg.GlobalRepliesSuffix = globalRepliesSuffix
	}
	domainCfg.globalBindID = uuid.New().String()
	domainCfg.globalRepliesID = calculateQueueName(domainCfg.Name, domainCfg.GlobalRepliesSuffix, true)

	rc := &ReactiveCommons{
		Domain:   domainCfg,
		rclient:  cli,
		registry: registry,
	}

	// Initialize components - pass pointer to rc.Domain so they all share the same instance
	rc.eventPublisher = NewEventPublisher(cli, rc.Domain)
	rc.eventListener = NewEventListener(cli, rc.Domain, registry)
	rc.commandSender = NewCommandSender(cli, rc.Domain)
	rc.commandReceiver = NewCommandReceiver(cli, rc.Domain, registry)
	rc.queryClient = NewQueryClient(cli, rc.Domain)
	rc.queryServer = NewQueryServer(cli, rc.Domain, registry)
	rc.topologyManager = NewTopologyManager(cli, &rc.Domain)

	return rc
}

var once sync.Once

func (rc *ReactiveCommons) Start(registry *Registry) {
	once.Do(func() {
		rc.Domain.UseDomainEvents = len(registry.EventHandlers) > 0
		rc.Domain.UseDirectQueries = len(registry.QueryHandlers) > 0
		rc.Domain.UseDirectCommands = len(registry.CommandHandlers) > 0 || len(registry.QueryHandlers) > 0

		rc.registry = registry

		// Update registry in components
		rc.eventListener.registry = registry
		rc.commandReceiver.registry = registry
		rc.queryServer.registry = registry

		err := rc.setupTopology()
		if err != nil {
			log.Panicf("Failed to setup topology: %v", err)
			return
		}

		for eventName, handler := range rc.registry.EventHandlers {
			rc.eventListener.ListenEvent(eventName, handler)
		}

		for commandName, handler := range rc.registry.CommandHandlers {
			rc.commandReceiver.HandleCommand(commandName, handler)
		}

		for resource, handler := range rc.registry.QueryHandlers {
			rc.queryServer.ServeQuery(resource, handler)
		}
	})
}

func (rc *ReactiveCommons) setupTopology() error {
	if rc.Domain.UseDomainEvents {
		err := rc.topologyManager.SetupDomainEvents()
		if err != nil {
			log.Printf("Failed to configure for domain events: %v", err)
			return err
		}
	}

	if rc.Domain.UseDirectCommands {
		err := rc.topologyManager.SetupDirectCommands()
		if err != nil {
			log.Printf("Failed to configure for direct commands: %v", err)
			return err
		}
	}

	if rc.Domain.UseDirectQueries {
		err := rc.topologyManager.SetupAsyncQueries()
		if err != nil {
			log.Printf("Failed to configure for async queries: %v", err)
			return err
		}
	}

	return nil
}

// -------------------------------
// Events - Delegate to components
// -------------------------------

func (rc *ReactiveCommons) EmitEvent(event DomainEvent[any], opts EventOptions) error {
	return rc.eventPublisher.EmitEvent(event, opts)
}

func (rc *ReactiveCommons) ListenEvents(handlers map[string]EventHandler) {
	rc.eventListener.ListenEvents(handlers)
}

func (rc *ReactiveCommons) ListenEvent(eventName string, handler EventHandler) {
	rc.eventListener.ListenEvent(eventName, handler)
}

// -------------------------------
// Commands - Delegate to components
// -------------------------------

func (rc *ReactiveCommons) SendCommand(command Command[any], opts CommandOptions) error {
	return rc.commandSender.SendCommand(command, opts)
}

func (rc *ReactiveCommons) HandleCommands(handlers map[string]CommandHandler) {
	rc.commandReceiver.HandleCommands(handlers)
}

func (rc *ReactiveCommons) HandleCommand(commandName string, handler CommandHandler) {
	rc.commandReceiver.HandleCommand(commandName, handler)
}

// -------------------------------
// Queries - Delegate to components
// -------------------------------

func (rc *ReactiveCommons) SendQueryRequest(request AsyncQuery[any], opts RequestReplyOptions) ([]byte, error) {
	return rc.queryClient.SendQueryRequest(request, opts)
}
