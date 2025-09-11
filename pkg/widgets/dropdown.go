package widgets

import (
	"errors"
	"fmt"

	"github.com/jroimartin/gocui"
)

type DropDown struct {
	*Panel
	Select   *Select
	ViewName string
	InputLen int
	Value    string
	Text     string

	// For multiselect dropdown
	multi bool
}

func NewDropDown(g *gocui.Gui, name, label string, getOptionsFunc GetOptionsFunc) (*DropDown, error) {
	maxX, maxY := g.Size()
	return &DropDown{
		Panel: &Panel{
			Name:    name,
			g:       g,
			Content: label,
			X0:      maxX / 8,
			Y0:      maxY / 8,
			X1:      maxX / 8 * 7,
			Y1:      maxY/8 + 3,
			KeyBindingTips: map[string]string{
				"TAB": "choose other options",
			},
		},
		Select: &Select{
			Panel: &Panel{
				Name:        name + "-dropdown-select",
				g:           g,
				KeyBindings: map[gocui.Key]func(*gocui.Gui, *gocui.View) error{},
				Frame:       true,
			},
			getOptionsFunc: getOptionsFunc,
		},
		ViewName: name + "-dropdown",
	}, nil
}

func (d *DropDown) SetMulti(multi bool) {
	d.multi = multi
	d.Select.SetMulti(true)
}

func (d *DropDown) Show() error {
	var err error
	if err = d.Panel.Show(); err != nil {
		return err
	}
	if err = d.Select.updateOptions(); err != nil {
		return err
	}
	offset := 20
	if d.Content == "" {
		offset = 0
	}
	if len(d.Content) > offset {
		offset = len(d.Content) + 1
	}
	x0 := d.X0 + offset
	x1 := d.X1 - 1
	y0 := d.Y0
	y1 := d.Y0 + 2
	d.InputLen = x1 - x0 - 1
	v, err := d.g.SetView(d.ViewName, x0, y0, x1, y1)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Highlight = true
		v.Wrap = true
		v.SelBgColor = gocui.ColorGreen
		v.SelFgColor = gocui.ColorBlack
		if d.Value == "" && d.Text == "" && d.Select.getOptionCount() > 0 {
			if d.multi {
				v.Highlight = false
			} else {
				opt, exists := d.Select.pickOptionByIndex(0)
				if !exists {
					return errors.New("should have at least one option")
				}
				d.Value = opt.Value
				d.Text = opt.Text
			}
		}
		err = d.g.SetKeybinding(d.ViewName, gocui.KeyTab, gocui.ModNone, func(_ *gocui.Gui, _ *gocui.View) error {
			d.Select.Value = d.Value
			d.Select.Panel.SetLocation(x0, y0, x1, y0+1)
			return d.Select.Show()
		})
		if err != nil {
			return err
		}
		d.Select.KeyBindings[gocui.KeyEnter] = func(g *gocui.Gui, v *gocui.View) error {
			if d.multi {
				// Append multiselect values
				d.Value = d.Select.Value
				d.Text = d.Select.Value
			} else {
				if len(v.BufferLines()) == 0 {
					return nil
				}
				_, cy := v.Cursor()
				if d.Select.getOptionCount() >= cy+1 {
					opt, exists := d.Select.pickOptionByIndex(cy)
					if !exists {
						return errors.New("should have at least one option")
					}
					d.Value = opt.Value
					d.Text = opt.Text
				}
			}
			if err = d.Select.Close(); err != nil {
				return err
			}
			if err = d.Close(); err != nil {
				return err
			}
			if err = d.Show(); err != nil {
				return err
			}
			return d.KeyBindings[gocui.KeyEnter](g, v)
		}
		d.Select.KeyBindings[gocui.KeyEsc] = func(_ *gocui.Gui, _ *gocui.View) error {
			if err = d.Select.Close(); err != nil {
				return err
			}
			if err = d.Close(); err != nil {
				return err
			}
			return d.Show()
		}
		if d.KeyBindings != nil {
			for key, f := range d.KeyBindings {
				if err = d.g.SetKeybinding(d.ViewName, key, gocui.ModNone, f); err != nil {
					return err
				}
			}
		}
	}
	if _, err = d.g.SetCurrentView(d.ViewName); err != nil {
		return err
	}

	return d.SetData(d.Value)
}

func (d *DropDown) Close() error {
	d.g.DeleteKeybindings(d.ViewName)
	if err := d.g.DeleteView(d.ViewName); err != nil {
		return err
	}
	return d.Panel.Close()
}

func (d *DropDown) GetData() (string, error) {
	return d.Value, nil
}

func (d *DropDown) GetMultiData() []string {
	return d.Select.GetMultiData()
}

func (d *DropDown) SetData(data string) error {
	if data == "" {
		d.Value = ""
		d.Text = ""
		return nil
	}

	var text string
	if d.multi {
		text = d.Value
	} else {
		if err := d.Select.updateOptions(); err != nil {
			return err
		}
		opt, exists := d.Select.pickOptionByValue(data)
		if !exists {
			return fmt.Errorf("given data '%s' not found in options", data)
		}
		d.Value = opt.Value
		d.Text = opt.Text
		text = opt.Text
	}

	v, err := d.g.View(d.ViewName)
	if err != nil {
		// Ignore ErrUnknownView for now
		if err == gocui.ErrUnknownView {
			return nil
		}
		return err
	}
	v.Clear()

	textLen := len(text)
	if d.InputLen > textLen {
		if _, err = v.Write([]byte(text)); err != nil {
			return err
		}
		for i := 0; i < d.InputLen-textLen-1; i++ {
			if _, err = v.Write([]byte{' '}); err != nil {
				return err
			}
		}
	} else {
		for i := 0; i < d.InputLen-1; i++ {
			if _, err = v.Write([]byte{text[i]}); err != nil {
				return err
			}
		}
	}
	if _, err = v.Write([]byte{'>'}); err != nil {
		return err
	}
	return nil
}

func (d *DropDown) PresetIfEmpty(value string) error {
	data, err := d.GetData()
	if err != nil {
		return err
	}
	if data != "" {
		return nil
	}
	return d.SetData(value)
}

func (d *DropDown) Reset() {
	d.Select.Reset()
	d.Value = ""
}
