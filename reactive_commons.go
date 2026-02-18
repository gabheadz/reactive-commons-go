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
	eventPublisher  *rabbitEventPublisher
	eventListener   *rabbitEventListener
	commandSender   *rabbitCommandSender
	commandReceiver *rabbitCommandReceiver
	queryClient     *rabbitQueryClient
	queryServer     *rabbitQueryServer
	topologyManager *rabbitTopologyManager
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
	rc.eventPublisher = newEventPublisher(cli, rc.Domain)
	rc.eventListener = newEventListener(cli, rc.Domain, registry)
	rc.commandSender = newCommandSender(cli, rc.Domain)
	rc.commandReceiver = newCommandReceiver(cli, rc.Domain, registry)
	rc.queryClient = newQueryClient(cli, rc.Domain)
	rc.queryServer = newQueryServer(cli, rc.Domain, registry)
	rc.topologyManager = newTopologyManager(cli, &rc.Domain)

	return rc
}

var once sync.Once

func (rc *ReactiveCommons) Start(registry *Registry) {
	// this can be called only once, subsequent calls will be ignored
	once.Do(func() {
		rc.Domain.UseDomainEvents = len(registry.EventHandlers) > 0
		rc.Domain.UseDirectQueries = len(registry.QueryHandlers) > 0
		rc.Domain.UseDirectCommands = len(registry.CommandHandlers) > 0 || len(registry.QueryHandlers) > 0

		rc.registry = registry

		// Set registry in components
		rc.eventListener.registry = registry
		rc.commandReceiver.registry = registry
		rc.queryServer.registry = registry

		err := rc.setupTopologyAndListeners()
		if err != nil {
			log.Panicf("Failed to setup topology: %v", err)
			return
		}
	})
}

func (rc *ReactiveCommons) setupTopologyAndListeners() error {
	if rc.Domain.UseDomainEvents {
		err := rc.topologyManager.setupDomainEvents()
		if err != nil {
			log.Printf("Failed to configure for domain events: %v", err)
			return err
		}
		rc.eventListener.setupBindings()
		rc.eventListener.startListeningEvents()
	}

	if rc.Domain.UseDirectCommands {
		err := rc.topologyManager.setupDirectCommands()
		if err != nil {
			log.Printf("Failed to configure for direct commands: %v", err)
			return err
		}
		rc.commandReceiver.startListeningCommands()
	}

	if rc.Domain.UseDirectQueries {
		err := rc.topologyManager.setupAsyncQueries()
		if err != nil {
			log.Printf("Failed to configure for async queries: %v", err)
			return err
		}
		rc.queryServer.serveQueries()
	}

	return nil
}

// -------------------------------
// Events - Delegate to components
// -------------------------------

func (rc *ReactiveCommons) EmitEvent(event DomainEvent[any], opts EventOptions) error {
	return rc.eventPublisher.emitEvent(event, opts)
}

// -------------------------------
// Commands - Delegate to components
// -------------------------------

func (rc *ReactiveCommons) SendCommand(command Command[any], opts CommandOptions) error {
	return rc.commandSender.sendCommand(command, opts)
}

// -------------------------------
// Queries - Delegate to components
// -------------------------------

func (rc *ReactiveCommons) SendQueryRequest(request AsyncQuery[any], opts RequestReplyOptions) ([]byte, error) {
	return rc.queryClient.sendQueryRequest(request, opts)
}
