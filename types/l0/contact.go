package l0

import (
	"fmt"

	"github.com/flocko-motion/ranke-workers/types"
)

// Contact is the normalized representation of a contact from any source
// (VCF, Google, WhatsApp, etc.).
type Contact struct {
	Name         string
	Emails       []types.Field
	Phones       []types.Field
	Addresses    []types.Field
	Organization string
	Title        string
	Birthday     string // YYYY-MM-DD or partial
	Notes        string // freeform body
}

// ToText renders the contact as text/ranke.
func (c Contact) ToText() string {
	doc := types.Document{Body: c.Notes}

	doc.Headers = append(doc.Headers, types.Field{Key: "Name", Value: c.Name})
	doc.Headers = append(doc.Headers, c.Emails...)
	doc.Headers = append(doc.Headers, c.Phones...)
	doc.Headers = append(doc.Headers, c.Addresses...)
	if c.Organization != "" {
		doc.Headers = append(doc.Headers, types.Field{Key: "Organization", Value: c.Organization})
	}
	if c.Title != "" {
		doc.Headers = append(doc.Headers, types.Field{Key: "Title", Value: c.Title})
	}
	if c.Birthday != "" {
		doc.Headers = append(doc.Headers, types.Field{Key: "Birthday", Value: c.Birthday})
	}

	return doc.Encode()
}

// ContactFromText parses a text/ranke encoded contact.
func ContactFromText(text string) (Contact, error) {
	doc := types.Decode(text)

	var c Contact
	c.Notes = doc.Body

	for _, f := range doc.Headers {
		switch f.Key {
		case "Name":
			c.Name = f.Value
		case "Email":
			c.Emails = append(c.Emails, f)
		case "Phone":
			c.Phones = append(c.Phones, f)
		case "Address":
			c.Addresses = append(c.Addresses, f)
		case "Organization":
			c.Organization = f.Value
		case "Title":
			c.Title = f.Value
		case "Birthday":
			c.Birthday = f.Value
		}
	}

	if c.Name == "" {
		return c, fmt.Errorf("contact missing Name field")
	}
	return c, nil
}
