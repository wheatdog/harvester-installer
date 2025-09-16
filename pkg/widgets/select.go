package widgets

import (
	"fmt"
	"strings"

	"github.com/jroimartin/gocui"
)

type Option struct {
	Value string
	Text  string
}

type OptionState struct {
	option   Option
	selected bool
}

type GetOptionsFunc func() ([]Option, error)

type Select struct {
	*Panel
	Value           string
	getOptionsFunc  GetOptionsFunc
	multi           bool
	optionStates    []OptionState
}

func NewSelect(g *gocui.Gui, name string, text string, getOptionsFunc GetOptionsFunc) (*Select, error) {
	return &Select{
		Panel: &Panel{
			Name:    name,
			g:       g,
			Content: text,
		},
		getOptionsFunc: getOptionsFunc,
	}, nil
}

func (s *Select) Show() error {
	var err error
	if err := s.Panel.Show(); err != nil {
		return err
	}
	if err := s.updateOptions(); err != nil {
		return err
	}
	optionViewName := s.Name + "-options"
	offset := 0
	if len(s.Content) > 0 {
		offset = len(strings.Split(s.Content, "\n")) + 1
	}
	y0 := s.Y0 + offset
	y1 := s.Y0 + offset + len(s.optionStates) + 1
	v, err := s.g.SetView(optionViewName, s.X0, y0, s.X1, y1)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		v.Wrap = true

		if s.multi {
			if err = s.updateSelectedStatus(v); err != nil {
				return err
			}
		} else {
			v.Highlight = true
			v.SelBgColor = gocui.ColorGreen
			v.SelFgColor = gocui.ColorBlack

			foundOptIdx := -1
			for idx, state := range s.optionStates {
				opt := state.option
				if _, err := fmt.Fprintln(v, opt.Text); err != nil {
					return err
				}
				if opt.Value == s.Value {
					foundOptIdx = idx
				}
			}

			// cursor should point to the current value if not empty
			if s.Value != "" {
				if foundOptIdx == -1 {
					return fmt.Errorf("'%s' not found in options", s.Value)
				}
				ox, oy := v.Origin()
				if err := v.SetCursor(ox, oy+foundOptIdx); err != nil {
					return err
				}
			}
		}

		if _, err := s.g.SetCurrentView(optionViewName); err != nil {
			return err
		}

		if err := s.setOptionsKeyBindings(optionViewName); err != nil {
			return err
		}
		if s.KeyBindings != nil {
			for key, f := range s.KeyBindings {
				if err := s.g.SetKeybinding(optionViewName, key, gocui.ModNone, f); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *Select) Close() error {
	optionViewName := s.Name + "-options"
	s.g.DeleteKeybindings(optionViewName)
	if err := s.g.DeleteView(optionViewName); err != nil {
		return err
	}
	return s.Panel.Close()
}

func (s *Select) SetMulti(multi bool) {
	s.multi = multi

	if multi {
		if len(s.KeyBindingTips) == 0 {
			s.KeyBindingTips = map[string]string{}
		}
		s.KeyBindingTips["SPACE"] = "select options"
	} else {
		delete(s.KeyBindingTips, "SPACE")
	}
}

func (s *Select) GetData() (string, error) {
	optionViewName := s.Name + "-options"
	ov, err := s.g.View(optionViewName)
	if err != nil {
		return "", err
	}
	if len(ov.BufferLines()) == 0 {
		return "", nil
	}
	_, cy := ov.Cursor()
	var value string
	if len(s.optionStates) >= cy+1 {
		value = s.optionStates[cy].option.Value
	}
	return value, nil
}

func (s *Select) GetMultiData() []string {
	selectedValues := make([]string, 0)
	for _, state := range s.optionStates {
		if state.selected {
			selectedValues = append(selectedValues, state.option.Value)
		}
	}
	return selectedValues
}

func (s *Select) Reset() {
	s.Value = ""
	s.optionStates = nil
}

func (s *Select) updateSelectedStatus(v *gocui.View) error {
	v.Clear()
	_, cy := v.Cursor()
	if err := v.SetCursor(1, cy); err != nil {
		return err
	}
	values := make([]string, 0)
	for _, state := range s.optionStates {
		selected := " "
		if state.selected {
			selected = "x"
			values = append(values, state.option.Value)
		}
		if _, err := fmt.Fprintf(v, "[%s] %s\n", selected, state.option.Text); err != nil {
			return err
		}
	}
	s.Value = strings.Join(values, ",")
	return nil
}

func (s *Select) setOptionsKeyBindings(viewName string) error {
	if err := setOptionsKeyBindings(s.g, viewName); err != nil {
		return err
	}
	if s.multi {
		handler := func(_ *gocui.Gui, v *gocui.View) error {
			_, cy := v.Cursor()
			if len(s.optionStates) >= cy+1 {
				s.optionStates[cy].selected = !s.optionStates[cy].selected
			}
			return s.updateSelectedStatus(v)
		}
		if err := s.g.SetKeybinding(viewName, gocui.KeySpace, gocui.ModNone, handler); err != nil {
			return err
		}
	}
	return nil
}

func setOptionsKeyBindings(g *gocui.Gui, viewName string) error {
	if err := g.SetKeybinding(viewName, gocui.KeyArrowUp, gocui.ModNone, ArrowUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(viewName, gocui.KeyArrowDown, gocui.ModNone, ArrowDown); err != nil {
		return err
	}
	return nil
}

func (s *Select) updateOptions() error {
	if s.getOptionsFunc == nil {
		return nil
	}

	options, err := s.getOptionsFunc();
	if err != nil {
		return err
	}

	selectedValues := make(map[string]struct{}, len(s.optionStates))
	for _, state := range(s.optionStates) {
		if state.selected {
			selectedValues[state.option.Value] = struct{}{}
		}
	}

	s.optionStates = make([]OptionState, 0, len(options))
	for _, opt := range(options) {
		_, selected := selectedValues[opt.Value]
		s.optionStates = append(s.optionStates, OptionState{option: opt, selected: selected})
	}

	return nil
}

func (s *Select) pickOptionByValue(value string) (Option, bool) {
	for _, state := range s.optionStates {
		opt := state.option
		if opt.Value == value {
			return opt, true
		}
	}
	return Option{}, false
}

func (s *Select) pickOptionByIndex(idx int) (Option, bool) {
	if idx < 0 || idx >= len(s.optionStates) {
		return Option{}, false
	}
	return s.optionStates[idx].option, true
}

func (s *Select) getOptionCount() int {
	return len(s.optionStates)
}
