package pool

type ID string

type Name string

// I don't know i have to bring Peer Available storage and other metric inside Peer
type Peer string

type ReplicationFactor int8

// Requirement of joining the pool
type Requirement struct {
	Latency   string `json:"latency"`
	Bandwidth string `json:"bandwidth"`
}

// ResourceRequest object (it is much more complicated for simplicity we think as file).
type ResourceRequest struct {
	ID  ID `json:"id"`
	FileName  Name `json:"filename"`
	Owner     Peer  `json:"owner"`
	Size      string `json:"size"`
	ReplicationFactor ReplicationFactor `json:"rp"`
}

type Resource struct{
	ResourceRequest
	Allocation []Peer 
}


// Event is a domain event marker.
type Event interface {
	isEvent()
}

func (e PoolCreated) isEvent() {}
func (e ResourceRequested) isEvent() {}
func (e ResourceAllocated) isEvent() {}
func (e MemberJoined) isEvent() {}
func (e MemberLeaved) isEvent() {}

// PoolCreated Event
type PoolCreated struct {
	ID	         ID           `json:"id"`
	Name         Name         `json:"name"`
	Creator      Peer         `json:"creator"`
	Members      []Peer       `json:"new_creator"`
	Requirement  Requirement  `json:"new_requirement"`
}

// ResourceRequested Event
type ResourceRequested struct {
	ID               ID               `json:"id"`
	ResourceRequest  ResourceRequest         `json:"new_resource"`
}

// ResourceAllocated Event
type ResourceAllocated struct {
	ID          ID          `json:"id"`
	Resource  ID  `json:"new_resource"`
	Allocation Peer
}

// MemberJoined Event
type MemberJoined struct {
	ID      ID    `json:"id"`
	Member  Peer  `json:"member"`
}

// MemberLeaved Event
type MemberLeaved struct {
	ID      ID    `json:"id"`
	Member  Peer  `json:"member"`
}


// Pool Aggregate
type Pool struct {
	id ID
	name Name
	creator Peer
	members []Peer
	requirement Requirement
	resources []Resource

	changes []Event
	version int
}

// NewFromEvents is a helper method that creates a new patient
// from a series of events.
func NewFromEvents(events []Event) *Pool {
	p := &Pool{}

	for _, event := range events {
		p.On(event, false)
	}

	return p
}

func (p *Pool) ID() ID {
	return p.id
}

func (p *Pool) Name() Name {
	return p.name
}

func (p *Pool) Creator() Peer {
	return p.creator
}

func (p *Pool) Members() []Peer {
	return p.members
}

func (p *Pool) Requirement() Requirement {
	return p.requirement
}

func (p *Pool) Allocated() []Resource {
	//TODO: have to go through resources and check if the replication factor is the same as len allocation
	return p.resources
}

func (p Pool) UnAllocated() []Resource{
	//TODO have to go through resources and check if the replication factor is the same as len allocation
	return p.resources 
}


// create new Pool from id, name, creator, members, requirement
func New(id ID, name Name, creator Peer, members []Peer, requirement Requirement) *Pool {
	p := &Pool{}
	p.raise(&PoolCreated{
		ID: id,
		Name: name,
		Creator: creator,
		Members: members,
		Requirement: requirement,
	})

	return p
}

// request network for new resource allocation
func (p *Pool) RequestResource(r ResourceRequest) error {
	//TODO check if the resource is already exist and allocation already satisfied
	
	p.raise(&ResourceRequested{
		ID: p.id,
		ResourceRequest: r,
	})

	return nil
}

// Allocated the resource to specific peer.
func (p *Pool) AllocateResource(resource ID, peer Peer) error {
	//TODO check if resource did not over allocated
	// Should i bring peer selection inside the model or just interface that it need?

	p.raise(&ResourceAllocated{
		ID: p.id,
		Resource: resource,
		Allocation: peer,
	})
	
	return nil
}

// Add peer to pool
func (p *Pool) Join(candidate Peer) error {
	// TODO: 
	// Should i bring peer condition and voting inside the model or just interface that it need?

	p.raise(&MemberJoined{
		Member: candidate,
		ID: p.id,
	})

	return nil
}

// Leave peer to pool
func (p *Pool) Leave(member Peer) error {
	// TODO: have to check all allocation the peer handled. Maybe create a specific Event for it .

	p.raise(&MemberLeaved{
		Member: member,
		ID: p.id,
	})

	return nil
}


// On handle pool event on pool aggregate.
func (p *Pool) On(event Event, new bool) {
	switch e := event.(type) {
	case *PoolCreated:
		p.id = e.ID
		p.creator = e.Creator
		p.members = e.Members
		p.name = e.Name
		p.requirement = e.Requirement
		p.resources = make([]Resource, 0)

	case *ResourceAllocated:
		for i,r := range p.resources {
			if r.ID == e.Resource {
				resource := p.resources[i]
				resource.Allocation = append(resource.Allocation, e.Allocation)
				p.resources[i] = resource
			}
		}

	case *ResourceRequested:
		r := &Resource{
			ResourceRequest: e.ResourceRequest,
			Allocation: make([]Peer, 0),

		}
		p.resources = append(p.resources, *r) 

	case *MemberJoined:
		p.members = append(p.members, e.Member)

	case *MemberLeaved:	
		fm := make([]Peer,0)
		for _,m := range p.members{
			if m != e.Member{
				fm = append(fm, m)
			}
		}
		p.members = fm
	}
	if !new {
		p.version++
	}
	
		 
}

// Events returns the uncommitted events from the pool aggregate.
func (p *Pool) Events() []Event {
	return p.changes
}

func (p *Pool) raise(event Event) {
	p.changes = append(p.changes, event)
	p.On(event, true)
}