import {act, fireEvent, render, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {
  DeleteBooking,
  DeleteProject,
  ExportBookings,
  GetState,
  RestoreProject,
  SaveBooking,
  SaveProject,
  PauseTimer,
  ResumeTimer,
  StopTimer,
  ToggleFavorite,
  ToggleTimer,
  SaveSettings,
  SavePropertyDefinition,
  MovePropertyDefinition,
  DeletePropertyDefinition,
  TestReminder,
  RunInBackground,
  QuitApplication,
  UpdateTimerStart,
  StartTemporaryTimer,
  AssignTemporaryTimer,
  DiscardTemporaryTimer,
} from '../wailsjs/go/main/App';
import {main, tracker} from '../wailsjs/go/models';
import {EventsOn, WindowSetDarkTheme, WindowSetLightTheme} from '../wailsjs/runtime/runtime';
import App from './App';
import wailsConfig from '../../wails.json';

vi.mock('../wailsjs/go/main/App', () => ({
  DeleteBooking: vi.fn(),
  DeleteProject: vi.fn(),
  ExportBookings: vi.fn(),
  GetState: vi.fn(),
  RestoreProject: vi.fn(),
  SaveBooking: vi.fn(),
  SaveProject: vi.fn(),
  PauseTimer: vi.fn(),
  ResumeTimer: vi.fn(),
  StopTimer: vi.fn(),
  ToggleFavorite: vi.fn(),
  ToggleTimer: vi.fn(),
  SaveSettings: vi.fn(),
  SavePropertyDefinition: vi.fn(),
  MovePropertyDefinition: vi.fn(),
  DeletePropertyDefinition: vi.fn(),
  TestReminder: vi.fn(),
  RunInBackground: vi.fn(),
  QuitApplication: vi.fn(),
  UpdateTimerStart: vi.fn(),
  StartTemporaryTimer: vi.fn(),
  AssignTemporaryTimer: vi.fn(),
  DiscardTemporaryTimer: vi.fn(),
}));

vi.mock('../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(() => vi.fn()),
  WindowSetDarkTheme: vi.fn(),
  WindowSetLightTheme: vi.fn(),
}));

const alpha = Object.assign(new tracker.Project(), {
  id: 'alpha',
  number: '100',
  name: 'Alpha',
  customer: 'Acme',
  parentId: '',
  color: '#123456',
  favorite: true,
  archived: false,
  properties: {},
});
const archived = Object.assign(new tracker.Project(), {
  id: 'archived',
  number: '200',
  name: 'Archived project',
  customer: 'Acme',
  parentId: '',
  color: '#654321',
  favorite: false,
  archived: true,
  properties: {},
});
const existingBooking = Object.assign(new main.Booking(), {
  id: 'booking-1',
  projectId: alpha.id,
  start: new Date(Date.now() - 3_600_000).toISOString(),
  end: new Date(Date.now() - 1_800_000).toISOString(),
  notes: 'Existing note',
});

function state(overrides: Partial<main.State> = {}): main.State {
  const initial = new main.State();
  initial.projects = [alpha];
  initial.bookings = [];
  initial.active = undefined;
  initial.pausedProjectId = '';
  initial.settings = Object.assign(new tracker.Settings(), {theme: 'light', reminders: defaultReminders()});
  initial.propertyDefinitions = [];
  return Object.assign(initial, overrides);
}

function defaultReminders() {
  return {runningEnabled: true, runningMinutes: 60, idleEnabled: true, idleMinutes: 30};
}

async function renderLoaded(initialState = state()) {
  vi.mocked(GetState).mockResolvedValue(initialState);
  render(<App />);
  await screen.findByRole('heading', {name: 'Time & calendar'});
}

async function selectProject(user: ReturnType<typeof userEvent.setup>, label: string, project: RegExp) {
  await user.click(screen.getByRole('combobox', {name: label}));
  await user.click(await screen.findByRole('button', {name: project}));
}

function requestClose() {
  const listener = vi.mocked(EventsOn).mock.calls.find(([name]) => name === 'close-requested')?.[1];
  if (!listener) throw new Error('Close event listener is not registered');
  act(() => listener());
}

describe('TimeManager workspace', () => {
  beforeEach(() => {
    vi.mocked(GetState).mockReset().mockResolvedValue(state());
    vi.mocked(DeleteBooking).mockReset().mockResolvedValue();
    vi.mocked(DeleteProject).mockReset().mockResolvedValue();
    vi.mocked(ExportBookings).mockReset().mockResolvedValue('');
    vi.mocked(RestoreProject).mockReset().mockResolvedValue();
    vi.mocked(SaveBooking).mockReset().mockResolvedValue();
    vi.mocked(SaveProject).mockReset().mockResolvedValue();
    vi.mocked(PauseTimer).mockReset().mockResolvedValue();
    vi.mocked(ResumeTimer).mockReset().mockResolvedValue();
    vi.mocked(StopTimer).mockReset().mockResolvedValue();
    vi.mocked(ToggleFavorite).mockReset().mockResolvedValue();
    vi.mocked(ToggleTimer).mockReset().mockResolvedValue();
    vi.mocked(EventsOn).mockClear();
    vi.mocked(SaveSettings).mockReset().mockResolvedValue();
    vi.mocked(SavePropertyDefinition).mockReset().mockResolvedValue();
    vi.mocked(MovePropertyDefinition).mockReset().mockResolvedValue();
    vi.mocked(DeletePropertyDefinition).mockReset().mockResolvedValue();
    vi.mocked(TestReminder).mockReset().mockResolvedValue();
    vi.mocked(RunInBackground).mockReset().mockResolvedValue();
    vi.mocked(QuitApplication).mockReset().mockResolvedValue();
    vi.mocked(UpdateTimerStart).mockReset().mockResolvedValue();
    vi.mocked(StartTemporaryTimer).mockReset().mockResolvedValue();
    vi.mocked(AssignTemporaryTimer).mockReset().mockResolvedValue();
    vi.mocked(DiscardTemporaryTimer).mockReset().mockResolvedValue();
    vi.mocked(WindowSetDarkTheme).mockClear();
    vi.mocked(WindowSetLightTheme).mockClear();
  });

  it.each(['favorites', 'directory'])('searches assigned custom property names and values in %s', async surface => {
    const user = userEvent.setup();
    const definitions = [
      {id: 'text', name: 'Contact', type: 'text', options: []},
      {id: 'number', name: 'Rate', type: 'number', options: []},
      {id: 'check', name: 'Billable', type: 'checkbox', options: []},
      {id: 'drop', name: 'Stage', type: 'dropdown', options: ['Closed']},
      {id: 'unset', name: 'Unset field', type: 'text', options: []},
    ];
    const enriched = {...alpha, properties: {text: 'Jane Smith', number: '125.5', check: 'false', drop: 'Closed', deleted: 'orphan'}};
    const other = {...alpha, id: 'other', name: 'Other', number: '300'};
    await renderLoaded(state({projects: [enriched, other, archived], propertyDefinitions: definitions}));
    if (surface === 'directory') await user.click(screen.getByRole('button', {name: 'Projects'}));
    const input = surface === 'directory'
      ? screen.getByRole('textbox', {name: 'Search projects'})
      : screen.getByRole('searchbox', {name: 'Search favorite projects'});
    for (const query of ['CONTACT', 'jAnE', '125.5', 'rate', 'billable', 'false', 'no', 'stage', 'closed']) {
      fireEvent.change(input, {target: {value: query}});
      expect(surface === 'directory'
        ? screen.getByText('100 - Alpha')
        : screen.getByRole('button', {name: 'Start timer for 100 Alpha'})).toBeInTheDocument();
      expect(surface === 'directory'
        ? screen.queryByText('300 - Other')
        : screen.queryByRole('button', {name: 'Start timer for 300 Other'})).not.toBeInTheDocument();
    }
    for (const query of ['unset field', 'orphan', 'true', 'yes']) {
      fireEvent.change(input, {target: {value: query}});
      expect(surface === 'directory'
        ? screen.queryByText('100 - Alpha')
        : screen.queryByRole('button', {name: 'Start timer for 100 Alpha'})).not.toBeInTheDocument();
    }
  });

  it('reorders project properties and keeps boundaries disabled', async () => {
    const user = userEvent.setup();
    const definitions = [
      {id: 'cost', name: 'Cost center', type: 'text', options: []},
      {id: 'rate', name: 'Hourly rate', type: 'number', options: []},
    ];
    await renderLoaded(state({propertyDefinitions: definitions}));
    await user.click(screen.getByRole('button', {name: /^Settings$/}));
    expect(screen.getByRole('button', {name: 'Move property Cost center up'})).toBeDisabled();
    expect(screen.getByRole('button', {name: 'Move property Cost center up'})).toHaveAttribute('title', 'This property is already first.');
    expect(screen.getByRole('button', {name: 'Move property Cost center down'})).not.toHaveAttribute('title');
    expect(screen.getByRole('button', {name: 'Move property Hourly rate down'})).toBeDisabled();
    expect(screen.getByRole('button', {name: 'Move property Hourly rate down'})).toHaveAttribute('title', 'This property is already last.');
    await user.click(screen.getByRole('button', {name: 'Edit property Cost center'}));
    expect(screen.getByRole('button', {name: 'Add property'})).toHaveAttribute('title', 'Finish or cancel the property you are editing first.');
    expect(screen.getByText('Property type').closest('label')).toHaveAttribute('title', 'Existing property types cannot be changed.');
    await user.click(screen.getByRole('button', {name: 'Cancel'}));
    expect(screen.getByRole('button', {name: 'Add property'})).toBeEnabled();
    vi.mocked(GetState).mockResolvedValue(state({propertyDefinitions: [...definitions].reverse()}));
    await user.click(screen.getByRole('button', {name: 'Move property Hourly rate up'}));
    expect(MovePropertyDefinition).toHaveBeenCalledWith('rate', -1);
    await waitFor(() => expect(screen.getByRole('button', {name: 'Move property Hourly rate up'})).toBeDisabled());
    const rows = screen.getAllByRole('row');
    expect(within(rows[1]).getByText('Hourly rate')).toBeInTheDocument();
    vi.mocked(MovePropertyDefinition).mockRejectedValueOnce(new Error('Could not save order'));
    await user.click(screen.getByRole('button', {name: 'Move property Hourly rate down'}));
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not save order');
    expect(within(screen.getAllByRole('row')[1]).getByText('Hourly rate')).toBeInTheDocument();
  });

  it('shows the application version from wails.json in settings', async () => {
    const user = userEvent.setup();
    await renderLoaded();
    await user.click(screen.getByRole('button', {name: /^Settings$/}));
    expect(__APP_VERSION__).toBe(wailsConfig.info.productVersion);
    expect(screen.getByText('TimeManager version', {exact: false})).toHaveTextContent(`TimeManager version ${wailsConfig.info.productVersion}`);
  });

  it('saves the temporary booking policy without changing other settings', async () => {
    const user = userEvent.setup();
    await renderLoaded();
    await user.click(screen.getByRole('button', {name: /^Settings$/}));
    expect(screen.getByLabelText('Temporary booking behavior')).toHaveValue('allow');
    await user.selectOptions(screen.getByLabelText('Temporary booking behavior'), 'require');
    expect(SaveSettings).toHaveBeenCalledWith(expect.objectContaining({
      requireTemporaryProject: true, theme: 'light', reminders: defaultReminders(),
    }));
  });

  it('shows ISO week-years in both calendar views and across New Year navigation', async () => {
    const user = userEvent.setup();
    await renderLoaded();
    fireEvent.change(screen.getByLabelText('Week of'), {target: {value: '2027-01-01'}});
    expect(screen.getByText('CW53')).toBeInTheDocument();
    expect(screen.getByText('CW53').parentElement).toHaveClass('calendar-date-range');
    expect(screen.getByLabelText('Week of')).not.toHaveAttribute('aria-describedby');
    await user.selectOptions(screen.getByLabelText('Calendar view'), 'summary');
    expect(screen.getByText('CW53')).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Next week'}));
    expect(screen.getByText('CW01')).toBeInTheDocument();
    await user.selectOptions(screen.getByLabelText('Calendar view'), 'details');
    expect(screen.getByText('CW01')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Week of'), {target: {value: '2024-12-30'}});
    expect(screen.getByText('CW01')).toBeInTheDocument();
  });

  it('loads the workspace and registers cleanup for Wails events', async () => {
    render(<App />);

    expect(await screen.findByRole('heading', {name: 'Time & calendar'})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Start timer for 100 Alpha'})).toBeEnabled();
    expect(EventsOn).toHaveBeenCalledWith('data-changed', expect.any(Function));
    expect(EventsOn).toHaveBeenCalledWith('app-error', expect.any(Function));
    expect(EventsOn).toHaveBeenCalledWith('close-requested', expect.any(Function));
  });

  it('starts temporary tracking and displays bridge failures', async () => {
    const user = userEvent.setup();
    await renderLoaded();
    await user.click(screen.getByRole('button', {name: 'Start temporary timer'}));
    expect(StartTemporaryTimer).toHaveBeenCalledOnce();
    vi.mocked(StartTemporaryTimer).mockRejectedValueOnce(new Error('Cannot save temporary timer'));
    await user.click(screen.getByRole('button', {name: 'Start temporary timer'}));
    expect(await screen.findByRole('alert')).toHaveTextContent('Cannot save temporary timer');
  });

  it('stops a temporary timer without requiring project assignment', async () => {
    const user = userEvent.setup();
    const temp = {...alpha, id: 'timemanager-temporary-project', name: 'Temporary / Unassigned'};
    const timer = Object.assign(new main.Timer(), {projectId: temp.id, start: new Date().toISOString()});
    await renderLoaded(state({projects: [alpha, temp], active: timer}));

    expect(screen.getByRole('group', {name: 'Timer actions'})).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Stop & save'}));

    expect(StopTimer).toHaveBeenCalledOnce();
    expect(AssignTemporaryTimer).not.toHaveBeenCalled();
    expect(screen.queryByRole('dialog', {name: 'Assign temporary time'})).not.toBeInTheDocument();
  });

  it('pauses and resumes timers through the timer panel', async () => {
    const user = userEvent.setup();
    const timer = Object.assign(new main.Timer(), {projectId: alpha.id, start: new Date(Date.now() - 60_000).toISOString()});
    await renderLoaded(state({active: timer}));
    const pausedState = state({pausedProjectId: alpha.id});
    vi.mocked(GetState).mockResolvedValueOnce(pausedState);
    await user.click(screen.getByRole('button', {name: 'Pause'}));
    expect(PauseTimer).toHaveBeenCalledOnce();

    await waitFor(() => expect(screen.getByText('TIMER PAUSED')).toBeInTheDocument());
    expect(screen.getByText(/resume to create a new booking segment/i)).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Resume'})).toBeEnabled();
    await user.click(screen.getByRole('button', {name: 'Resume'}));
    expect(ResumeTimer).toHaveBeenCalledOnce();
  });

  it('sets the project of a running temporary session without stopping it', async () => {
    const user = userEvent.setup();
    const temp = {...alpha, id: 'timemanager-temporary-project', name: 'Temporary / Unassigned'};
    const timer = Object.assign(new main.Timer(), {projectId: temp.id, start: '2026-11-03T11:00:00.1234567Z'});
    await renderLoaded(state({projects: [alpha, temp], active: timer}));
    await user.click(screen.getByRole('button', {name: 'Set project'}));
    const dialog = screen.getByRole('dialog', {name: 'Assign temporary time'});
    expect(within(dialog).getByText(/original start time; no booking is created/)).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', {name: 'Cancel'}));
    expect(AssignTemporaryTimer).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', {name: 'Set project'}));
    await selectProject(user, 'Assign to project', /100 - Alpha/);
    await user.click(screen.getByRole('button', {name: 'Assign & continue'}));
    expect(AssignTemporaryTimer).toHaveBeenCalledWith({timer, action: 'assign-running', id: ''}, alpha.id);
    expect(StopTimer).not.toHaveBeenCalled();
    expect(ToggleTimer).not.toHaveBeenCalled();
  });

  it.each(['stop', 'pause', 'toggle', 'favorite', 'start', 'quit'])('assigns temporary time before completing %s', async action => {
    const user = userEvent.setup();
    const temp = {...alpha, id: 'timemanager-temporary-project', name: 'Temporary / Unassigned', number: 'TEMP'};
    const timer = Object.assign(new main.Timer(), {projectId: temp.id, start: new Date().toISOString()});
    await renderLoaded(state({projects: [alpha, archived, temp], active: timer}));
    const request = Object.assign(new main.TemporaryAssignment(), {timer, action, id: ['toggle', 'favorite', 'start'].includes(action) ? alpha.id : ''});
    const listener = vi.mocked(EventsOn).mock.calls.find(([name]) => name === 'temporary-assignment-requested')?.[1];
    if (!listener) throw new Error('Temporary assignment listener missing');
    act(() => listener(request));
    const dialog = screen.getByRole('dialog', {name: 'Assign temporary time'});
    expect(within(dialog).getByRole('button', {name: 'Assign & continue'})).toBeDisabled();
    await user.click(within(dialog).getByRole('combobox', {name: 'Assign to project'}));
    const projectChoices = screen.getByRole('list', {name: 'Assign to project hierarchy'});
    expect(within(projectChoices).queryByRole('button', {name: /Archived/})).not.toBeInTheDocument();
    expect(within(projectChoices).queryByRole('button', {name: /Temporary/})).not.toBeInTheDocument();
    await user.click(within(projectChoices).getByRole('button', {name: /100 - Alpha/}));
    await user.click(within(dialog).getByRole('button', {name: 'Assign & continue'}));
    expect(AssignTemporaryTimer).toHaveBeenCalledWith(request, alpha.id);
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
  });

  it.each(['stop', 'pause', 'toggle', 'quit', 'assign-running'])('discards temporary time without assignment for %s', async action => {
    const user = userEvent.setup();
    const timer = Object.assign(new main.Timer(), {projectId: 'timemanager-temporary-project', start: new Date().toISOString()});
    await renderLoaded(state({projects: [], active: timer}));
    const listener = vi.mocked(EventsOn).mock.calls.find(([name]) => name === 'temporary-assignment-requested')?.[1];
    if (!listener) throw new Error('Temporary assignment listener missing');
    act(() => listener({timer, action, id: ''}));
    const dialog = screen.getByRole('dialog', {name: 'Assign temporary time'});
    await user.click(within(dialog).getByRole('button', {name: 'Cancel booking'}));
    expect(DiscardTemporaryTimer).toHaveBeenCalledWith(timer);
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(AssignTemporaryTimer).not.toHaveBeenCalled();
    expect(QuitApplication).not.toHaveBeenCalled();
    expect(ToggleTimer).not.toHaveBeenCalled();
  });

  it('keeps discard errors visible without closing the assignment dialog', async () => {
    const user = userEvent.setup();
    await renderLoaded();
    const timer = Object.assign(new main.Timer(), {projectId: 'timemanager-temporary-project', start: new Date().toISOString()});
    const listener = vi.mocked(EventsOn).mock.calls.find(([name]) => name === 'temporary-assignment-requested')?.[1];
    if (!listener) throw new Error('Temporary assignment listener missing');
    act(() => listener({timer, action: 'stop', id: ''}));
    vi.mocked(DiscardTemporaryTimer).mockRejectedValueOnce(new Error('Discard could not be saved'));
    const dialog = screen.getByRole('dialog', {name: 'Assign temporary time'});
    await user.click(within(dialog).getByRole('button', {name: 'Cancel booking'}));
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('Discard could not be saved');
    expect(dialog).toBeInTheDocument();
  });

  it('keeps temporary assignment errors visible and lets Cancel keep tracking', async () => {
    const user = userEvent.setup();
    const timer = Object.assign(new main.Timer(), {projectId: 'timemanager-temporary-project', start: new Date().toISOString()});
    await renderLoaded();
    const request = Object.assign(new main.TemporaryAssignment(), {timer, action: 'quit', id: ''});
    const listener = vi.mocked(EventsOn).mock.calls.find(([name]) => name === 'temporary-assignment-requested')?.[1];
    if (!listener) throw new Error('Temporary assignment listener missing');
    act(() => listener(request));
    const dialog = screen.getByRole('dialog', {name: 'Assign temporary time'});
    await selectProject(user, 'Assign to project', /100 - Alpha/);
    vi.mocked(AssignTemporaryTimer).mockRejectedValueOnce(new Error('Save failed; timer unchanged'));
    await user.click(within(dialog).getByRole('button', {name: 'Assign & continue'}));
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('Save failed; timer unchanged');
    await user.click(within(dialog).getByRole('button', {name: 'Cancel'}));
    expect(dialog).not.toBeInTheDocument();
    expect(StopTimer).not.toHaveBeenCalled();
    expect(QuitApplication).not.toHaveBeenCalled();
  });

  it('explains how to assign temporary time when no other project exists', async () => {
    const user = userEvent.setup();
    const temp = {...alpha, id: 'timemanager-temporary-project', name: 'Temporary / Unassigned'};
    const timer = Object.assign(new main.Timer(), {projectId: temp.id, start: new Date().toISOString()});
    await renderLoaded(state({projects: [temp], active: timer}));
    const listener = vi.mocked(EventsOn).mock.calls.find(([name]) => name === 'temporary-assignment-requested')?.[1];
    if (!listener) throw new Error('Temporary assignment listener missing');
    act(() => listener({timer, action: 'stop', id: ''}));
    expect(screen.getByText('No project is available. Cancel and create a project first.')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Assign & continue'})).toBeDisabled();
    await user.click(screen.getByRole('button', {name: 'Cancel'}));
    expect(AssignTemporaryTimer).not.toHaveBeenCalled();
    expect(StopTimer).not.toHaveBeenCalled();
  });

  it('shows a load error and retries the state request', async () => {
    const user = userEvent.setup();
    vi.mocked(GetState)
      .mockRejectedValueOnce(new Error('Data file is unavailable'))
      .mockResolvedValueOnce(state());
    render(<App />);

    expect(await screen.findByText('Could not load your saved data.')).toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('Data file is unavailable');
    await user.click(screen.getByRole('button', {name: 'Retry'}));

    expect(await screen.findByRole('heading', {name: 'Time & calendar'})).toBeInTheDocument();
    expect(GetState).toHaveBeenCalled();
  });

  it('starts and stops a timer, refreshing the active state after each action', async () => {
    const user = userEvent.setup();
    vi.mocked(GetState)
      .mockResolvedValueOnce(state())
      .mockResolvedValueOnce(state({active: Object.assign(new main.Timer(), {
        projectId: alpha.id,
        start: new Date().toISOString(),
      })}))
      .mockResolvedValueOnce(state());
    render(<App />);

    await user.click(await screen.findByRole('button', {name: 'Start timer for 100 Alpha'}));
    expect(ToggleTimer).toHaveBeenCalledWith(alpha.id);
    expect(await screen.findByText('CURRENTLY TRACKING')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Stop & save'})).toBeEnabled();

    await user.click(screen.getByRole('button', {name: 'Stop & save'}));
    expect(StopTimer).toHaveBeenCalledOnce();
    await waitFor(() => expect(screen.getByText('NO TIMER RUNNING')).toBeInTheDocument());
  });

  it('moves running highlights and accessible pressed state when switching and stopping timers', async () => {
    const user = userEvent.setup();
    const beta = {...alpha, id: 'beta', number: '101', name: 'Beta'};
    const projects = [alpha, beta];
    await renderLoaded(state({projects, active: Object.assign(new main.Timer(), {projectId: alpha.id, start: new Date().toISOString()})}));
    const alphaCard = screen.getByRole('button', {name: 'Stop timer for 100 Alpha'});
    expect(alphaCard).toHaveClass('active');
    expect(alphaCard).toHaveAttribute('aria-pressed', 'true');
    expect(within(alphaCard).getByText('RUNNING')).toHaveClass('running-badge');
    expect(alphaCard.querySelector('.running-spinner')).toHaveAttribute('aria-hidden', 'true');
    expect(screen.getByRole('button', {name: 'Start timer for 101 Beta'}).querySelector('.running-spinner')).toBeNull();
    expect(screen.getByRole('button', {name: 'Start timer for 101 Beta'})).toHaveAttribute('aria-pressed', 'false');
    vi.mocked(GetState).mockResolvedValue(state({projects, active: Object.assign(new main.Timer(), {projectId: beta.id, start: new Date().toISOString()})}));
    await user.click(screen.getByRole('button', {name: 'Start timer for 101 Beta'}));
    const betaCard = await screen.findByRole('button', {name: 'Stop timer for 101 Beta'});
    expect(betaCard).toHaveClass('active');
    expect(screen.getByRole('button', {name: 'Start timer for 100 Alpha'})).not.toHaveClass('active');
    await user.click(screen.getByRole('button', {name: 'Projects'}));
    const directory = screen.getByRole('list', {name: 'Project directory hierarchy'});
    const alphaRow = within(directory).getByText('100 - Alpha').closest('.directory-row');
    const betaRow = within(directory).getByText('101 - Beta').closest('.directory-row');
    if (!(alphaRow instanceof HTMLElement) || !(betaRow instanceof HTMLElement)) throw new Error('Project rows missing');
    expect(betaRow).toHaveClass('running-project');
    expect(within(betaRow).getByText('RUNNING')).toHaveClass('running-badge');
    expect(betaRow.querySelector('.running-spinner')).toBeInTheDocument();
    expect(alphaRow.querySelector('.running-spinner')).toBeNull();
    expect(alphaRow).not.toHaveClass('running-project');
    vi.mocked(GetState).mockResolvedValue(state({projects}));
    await user.click(screen.getByRole('button', {name: 'Stop & save'}));
    await waitFor(() => expect(betaRow).not.toHaveClass('running-project'));
    expect(screen.queryByText('RUNNING')).not.toBeInTheDocument();
    expect(document.querySelector('.running-spinner')).toBeNull();
  });

  it('highlights a running non-favorite project in the directory', async () => {
    const user = userEvent.setup();
    const project = {...alpha, favorite: false};
    await renderLoaded(state({projects: [project], active: Object.assign(new main.Timer(), {projectId: project.id, start: new Date().toISOString()})}));
    await user.click(screen.getByRole('button', {name: 'Projects'}));
    const directory = screen.getByRole('list', {name: 'Project directory hierarchy'});
    const row = within(directory).getByText('100 - Alpha').closest('.directory-row');
    if (!(row instanceof HTMLElement)) throw new Error('Project row missing');
    expect(row).toHaveClass('running-project');
    expect(within(row).getByText('RUNNING')).toBeInTheDocument();
  });

  it('displays subprojects inside their parent project tree branch', async () => {
    const user = userEvent.setup();
    const child = {...alpha, id: 'child', number: '101', name: 'Child', favorite: false, parentId: alpha.id};
    await renderLoaded(state({projects: [alpha, child]}));
    await user.click(screen.getByRole('button', {name: 'Projects'}));

    const directory = screen.getByRole('list', {name: 'Project directory hierarchy'});
    const parentBranch = within(directory).getByRole('button', {name: 'Collapse 100 Alpha'}).closest('li');
    if (!(parentBranch instanceof HTMLElement)) throw new Error('Parent project branch missing');
    expect(within(parentBranch).getByText('101 - Child')).toBeInTheDocument();
  });

  it.each(['101', '  bEtA  ', 'gLoBeX', 'Umbrella'])('filters favorites by number, name, customer, and parent hierarchy (%s)', async (query) => {
    const user = userEvent.setup();
    const parent = {...alpha, id: 'parent', number: '10', name: 'Umbrella', favorite: false};
    const beta = {...alpha, id: 'beta', number: '101', name: 'Beta', customer: 'Globex', parentId: parent.id};
    await renderLoaded(state({projects: [alpha, parent, beta, archived]}));
    await user.type(screen.getByRole('searchbox', {name: 'Search favorite projects'}), query);
    expect(screen.getByRole('button', {name: 'Start timer for 101 Beta'})).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Start timer for 100 Alpha'})).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Start timer for 10 Umbrella'})).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Start timer for 200 Archived project'})).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Clear favorite search'}));
    expect(screen.getByRole('searchbox', {name: 'Search favorite projects'})).toHaveValue('');
    expect(screen.getByRole('button', {name: 'Start timer for 100 Alpha'})).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Start timer for 101 Beta'}));
    expect(ToggleTimer).toHaveBeenCalledWith(beta.id);
    expect(ToggleFavorite).not.toHaveBeenCalled();
    expect(SaveProject).not.toHaveBeenCalled();
  });

  it('shows a no-match message without hiding the running timer or calendar', async () => {
    const user = userEvent.setup();
    await renderLoaded(state({active: Object.assign(new main.Timer(), {
      projectId: alpha.id, start: new Date().toISOString(),
    })}));
    await user.type(screen.getByRole('searchbox', {name: 'Search favorite projects'}), 'missing');
    expect(screen.getByRole('heading', {name: 'No favorite projects match your search'})).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Stop timer for 100 Alpha'})).not.toBeInTheDocument();
    expect(screen.getByText('CURRENTLY TRACKING')).toBeInTheDocument();
    expect(screen.getByRole('heading', {name: 'Booking calendar'})).toBeInTheDocument();
    expect(StopTimer).not.toHaveBeenCalled();
    expect(ToggleTimer).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', {name: 'Stop & save'}));
    expect(StopTimer).toHaveBeenCalledOnce();
  });

  it('keeps favorites search independent of directory search across navigation', async () => {
    const user = userEvent.setup();
    const beta = {...alpha, id: 'beta', number: '101', name: 'Beta'};
    await renderLoaded(state({projects: [alpha, beta]}));
    await user.type(screen.getByRole('searchbox', {name: 'Search favorite projects'}), 'beta');
    await user.click(screen.getByRole('button', {name: 'Projects'}));
    expect(screen.getByRole('textbox', {name: 'Search projects'})).toHaveValue('');
    await user.type(screen.getByRole('textbox', {name: 'Search projects'}), 'alpha');
    expect(screen.queryByText('101 - Beta')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Time & calendar'}));
    expect(screen.getByRole('searchbox', {name: 'Search favorite projects'})).toHaveValue('beta');
    expect(screen.getByRole('button', {name: 'Start timer for 101 Beta'})).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Start timer for 100 Alpha'})).not.toBeInTheDocument();
  });

  it('preserves the empty-favorites guidance and ignores whitespace-only searches', async () => {
    const user = userEvent.setup();
    await renderLoaded(state({projects: [{...alpha, favorite: false}, archived]}));
    await user.type(screen.getByRole('searchbox', {name: 'Search favorite projects'}), 'missing');
    expect(screen.getByRole('heading', {name: 'Your favorites go here'})).toBeInTheDocument();
    expect(screen.queryByText('No favorite projects match your search')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Clear favorite search'}));
    const listener = vi.mocked(EventsOn).mock.calls.find(([name]) => name === 'data-changed')?.[1];
    if (!listener) throw new Error('Data change listener is not registered');
    vi.mocked(GetState).mockResolvedValue(state());
    await act(async () => listener());
    await user.type(screen.getByRole('searchbox', {name: 'Search favorite projects'}), '   ');
    expect(await screen.findByRole('button', {name: 'Start timer for 100 Alpha'})).toBeInTheDocument();
  });

  it('edits the running start, preserves untouched timestamp precision, and cancels without saving', async () => {
    const user = userEvent.setup();
    const active = Object.assign(new main.Timer(), {projectId: alpha.id, start: '2026-01-01T09:00:00.123456789Z'});
    await renderLoaded(state({active}));
    await user.click(screen.getByRole('button', {name: 'Edit start time'}));
    await user.click(screen.getByRole('button', {name: 'Cancel'}));
    expect(UpdateTimerStart).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', {name: 'Edit start time'}));
    await user.click(screen.getByRole('button', {name: 'Save start time'}));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(UpdateTimerStart).toHaveBeenCalledExactlyOnceWith(active, active.start);
    expect(StopTimer).not.toHaveBeenCalled();
    expect(SaveBooking).not.toHaveBeenCalled();
  });

  it('saves an edited local start time and refreshes the running timer', async () => {
    const user = userEvent.setup();
    const active = Object.assign(new main.Timer(), {projectId: alpha.id, start: '2026-01-01T09:00:00Z'});
    await renderLoaded(state({active}));
    await user.click(screen.getByRole('button', {name: 'Edit start time'}));
    const input = screen.getByLabelText('Timer start time');
    if (!(input instanceof HTMLInputElement)) throw new Error('Timer start is not an input');
    const next = '2026-01-01T08:30:00';
    fireEvent.change(input, {target: {value: '2026-01-01'}});
    fireEvent.change(screen.getByLabelText('Time for Timer start time'), {target: {value: '08:30:00'}});
    const saved = new Date(next).toISOString();
    vi.mocked(GetState).mockResolvedValue(state({active: Object.assign(new main.Timer(), {projectId: active.projectId, start: saved})}));
    await user.click(screen.getByRole('button', {name: 'Save start time'}));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(UpdateTimerStart).toHaveBeenCalledExactlyOnceWith(active, saved);
    expect(screen.getByText('CURRENTLY TRACKING')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Stop & save'})).toBeInTheDocument();
  });

  it('keeps timer start editor open with errors and prevents duplicate saves', async () => {
    const user = userEvent.setup();
    const active = Object.assign(new main.Timer(), {projectId: alpha.id, start: '2026-01-01T09:00:00Z'});
    await renderLoaded(state({active}));
    await user.click(screen.getByRole('button', {name: 'Edit start time'}));
    let rejectSave: (error: Error) => void = () => {throw new Error('Save promise not initialized');};
    vi.mocked(UpdateTimerStart).mockImplementationOnce(() => new Promise<void>((_resolve, reject) => {rejectSave = reject;}));
    await user.click(screen.getByRole('button', {name: 'Save start time'}));
    expect(screen.getByRole('button', {name: 'Save start time'})).toBeDisabled();
    await user.click(screen.getByRole('button', {name: 'Save start time'}));
    expect(UpdateTimerStart).toHaveBeenCalledOnce();
    await act(async () => rejectSave(new Error('running timer changed; reopen the start time editor')));
    expect(await screen.findByRole('alert')).toHaveTextContent('running timer changed');
    expect(screen.getByRole('dialog', {name: 'Edit timer start time'})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Save start time'})).toBeEnabled();
  });

  it('surfaces action failures and dismisses the error banner', async () => {
    const user = userEvent.setup();
    vi.mocked(ToggleTimer).mockRejectedValueOnce('Timer could not start');
    await renderLoaded();

    await user.click(await screen.findByRole('button', {name: 'Start timer for 100 Alpha'}));
    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Timer could not start');
    await user.click(within(alert).getByRole('button', {name: 'Dismiss'}));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('searches projects and toggles archived projects in the directory', async () => {
    const user = userEvent.setup();
    await renderLoaded(state({projects: [alpha, archived]}));

    await user.click(screen.getByRole('button', {name: 'Projects'}));
    expect(screen.getByRole('heading', {name: 'Project directory'})).toBeInTheDocument();
    expect(screen.queryByText('200 - Archived project')).not.toBeInTheDocument();

    await user.click(screen.getByLabelText('Show archived'));
    expect(screen.getByText('200 - Archived project')).toBeInTheDocument();
    await user.type(screen.getByRole('textbox', {name: 'Search projects'}), 'missing');
    expect(screen.getByText('No projects here yet')).toBeInTheDocument();
  });

  it('distinguishes favorite actions for projects sharing a number', async () => {
    const user = userEvent.setup();
    const beta = {...alpha, id: 'beta', name: 'Beta'};
    await renderLoaded(state({projects: [alpha, beta]}));
    await user.click(screen.getByRole('button', {name: 'Start timer for 100 Beta'}));
    expect(ToggleTimer).toHaveBeenCalledWith(beta.id);
    await user.click(screen.getByRole('button', {name: 'Projects'}));
    await user.click(screen.getByRole('button', {name: 'Remove favorite 100 Beta'}));
    expect(ToggleFavorite).toHaveBeenCalledExactlyOnceWith(beta.id);
    expect(screen.getByRole('button', {name: 'Remove favorite 100 Alpha'})).toBeInTheDocument();
  });

  it('creates a project with the entered fields', async () => {
    const user = userEvent.setup();
    await renderLoaded();

    await user.click(screen.getByRole('button', {name: 'Projects'}));
    await user.click(screen.getByRole('button', {name: 'Create project'}));
    await user.type(screen.getByLabelText('Project number'), '300');
    await user.type(screen.getByLabelText('Customer'), 'Globex');
    await user.type(screen.getByLabelText('Project name'), 'Migration');
    await user.click(screen.getByRole('button', {name: 'Save project'}));

    await waitFor(() => expect(SaveProject).toHaveBeenCalledOnce());
    expect(SaveProject).toHaveBeenCalledWith(expect.objectContaining({
      number: '300',
      name: 'Migration',
      customer: 'Globex',
      parentId: '',
      favorite: true,
      archived: false,
    }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });

  it('copies project details into an editable new project without copying time or children', async () => {
    const user = userEvent.setup();
    const source = {...alpha, parentId: 'parent', favorite: false, properties: {cost: 'CC-42'}};
    const parent = {...alpha, id: 'parent', number: '10', name: 'Parent'};
    const child = {...alpha, id: 'child', number: '101', name: 'Child', parentId: source.id};
    const initialState = state({
      projects: [parent, source, child],
      bookings: [existingBooking],
      active: Object.assign(new main.Timer(), {projectId: source.id, start: new Date().toISOString()}),
      propertyDefinitions: [{id: 'cost', name: 'Cost center', type: 'text', options: []}],
    });
    await renderLoaded(initialState);
    await user.click(screen.getByRole('button', {name: 'Projects'}));
    await user.click(screen.getByRole('button', {name: 'Copy project 100 Alpha'}));
    const dialog = screen.getByRole('dialog', {name: 'Create project'});
    expect(within(dialog).getByLabelText('Project number')).toHaveValue('100');
    expect(within(dialog).getByLabelText('Project name')).toHaveValue('');
    expect(within(dialog).getByLabelText('Customer')).toHaveValue('Acme');
    expect(within(dialog).getByLabelText('Project color')).toHaveValue('#123456');
    expect(within(dialog).getByRole('combobox', {name: 'Parent project'})).toHaveTextContent('10 - Parent');
    expect(within(dialog).getByLabelText('Add to favorites')).not.toBeChecked();
    expect(within(dialog).getByLabelText('Cost center')).toHaveValue('CC-42');
    await user.click(within(dialog).getByRole('button', {name: 'Save project'}));
    expect(SaveProject).not.toHaveBeenCalled();
    await user.type(within(dialog).getByLabelText('Project name'), 'Alpha copy');
    await user.clear(within(dialog).getByLabelText('Cost center'));
    await user.type(within(dialog).getByLabelText('Cost center'), 'CC-99');
    await user.click(within(dialog).getByRole('button', {name: 'Save project'}));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(SaveProject).toHaveBeenCalledExactlyOnceWith({
      ...source, id: '', name: 'Alpha copy', archived: false, properties: {cost: 'CC-99'},
    });
    expect(source.properties).toEqual({cost: 'CC-42'});
    expect(initialState.projects).toEqual([parent, source, child]);
    expect(initialState.bookings).toEqual([existingBooking]);
    expect(initialState.active?.projectId).toBe(source.id);
    expect(SaveBooking).not.toHaveBeenCalled();
    expect(ToggleTimer).not.toHaveBeenCalled();
    expect(StopTimer).not.toHaveBeenCalled();
  });

  it.each([false, true])('copies archived projects as active and clears only archived parents (archived parent: %s)', async (archivedParent) => {
    const user = userEvent.setup();
    const parent = {...alpha, id: 'parent', number: '10', archived: archivedParent, favorite: false};
    const source = {...archived, parentId: parent.id};
    await renderLoaded(state({projects: [parent, source]}));
    await user.click(screen.getByRole('button', {name: 'Projects'}));
    await user.click(screen.getByLabelText('Show archived'));
    await user.click(screen.getByRole('button', {name: 'Copy project 200 Archived project'}));
    expect(screen.getByRole('combobox', {name: 'Parent project'})).toHaveTextContent(
      archivedParent ? 'None (top-level project)' : '10 - Alpha',
    );
    expect(screen.getByLabelText('Add to favorites')).toBeEnabled();
    await user.type(screen.getByLabelText('Project name'), 'Archived copy');
    await user.click(screen.getByRole('button', {name: 'Save project'}));
    await waitFor(() => expect(SaveProject).toHaveBeenCalledExactlyOnceWith({
      ...source, id: '', name: 'Archived copy', archived: false, parentId: archivedParent ? '' : parent.id,
    }));
  });

  it('cancels copying without saving and keeps duplicate-name errors in the editor', async () => {
    const user = userEvent.setup();
    await renderLoaded();
    await user.click(screen.getByRole('button', {name: 'Projects'}));
    await user.click(screen.getByRole('button', {name: 'Copy project 100 Alpha'}));
    expect(screen.getByLabelText('Add to favorites')).toBeChecked();
    expect(screen.getByLabelText('Parent project')).toHaveValue('');
    await user.click(screen.getByRole('button', {name: 'Cancel'}));
    expect(SaveProject).not.toHaveBeenCalled();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    vi.mocked(SaveProject).mockRejectedValueOnce(new Error('project names must be unique (including archived projects)'));
    await user.click(screen.getByRole('button', {name: 'Copy project 100 Alpha'}));
    await user.type(screen.getByLabelText('Project name'), alpha.name);
    await user.click(screen.getByRole('button', {name: 'Save project'}));
    expect(await screen.findByRole('alert')).toHaveTextContent('project names must be unique');
    expect(screen.getByRole('dialog', {name: 'Create project'})).toBeInTheDocument();
    expect(screen.getByLabelText('Project number')).toHaveValue(alpha.number);
    await user.clear(screen.getByLabelText('Project name'));
    await user.type(screen.getByLabelText('Project name'), 'Alpha copy');
    await user.click(screen.getByRole('button', {name: 'Save project'}));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(SaveProject).toHaveBeenLastCalledWith({...alpha, id: '', name: 'Alpha copy'});
  });

  it('toggles a favorite, archives a booked project, and restores archived projects', async () => {
    const user = userEvent.setup();
    vi.mocked(GetState)
      .mockResolvedValueOnce(state({bookings: [existingBooking]}))
      .mockResolvedValueOnce(state({bookings: [existingBooking]}))
      .mockResolvedValueOnce(state({projects: [alpha, archived], bookings: [existingBooking]}));
    render(<App />);
    await screen.findByRole('heading', {name: 'Time & calendar'});

    await user.click(screen.getByRole('button', {name: 'Projects'}));
    await user.click(await screen.findByRole('button', {name: 'Remove favorite 100 Alpha'}));
    expect(ToggleFavorite).toHaveBeenCalledWith(alpha.id);

    await user.click(screen.getByRole('button', {name: 'Archive'}));
    const confirmation = screen.getByRole('dialog', {name: 'Archive project?'});
    await user.click(within(confirmation).getByRole('button', {name: 'Confirm'}));
    expect(DeleteProject).toHaveBeenCalledWith(alpha.id);

    await user.click(screen.getByLabelText('Show archived'));
    await user.click(await screen.findByRole('button', {name: 'Restore'}));
    expect(RestoreProject).toHaveBeenCalledWith(archived.id);
  });

  it.each([false, true])('asks before closing with a background option (timer running: %s)', async (running) => {
    const user = userEvent.setup();
    await renderLoaded(state({active: running ? Object.assign(new main.Timer(), {
      projectId: alpha.id, start: new Date().toISOString(),
    }) : undefined}));
    requestClose();
    const dialog = screen.getByRole('dialog', {name: 'Close TimeManager?'});
    expect(within(dialog).queryByRole('button', {name: 'Close dialog'})).not.toBeInTheDocument();
    expect(within(dialog).getByRole('button', {name: 'Run in background'})).toBeEnabled();
    expect(within(dialog).getByRole('button', {name: 'Quit'})).toBeEnabled();
    await user.click(within(dialog).getByRole('button', {name: 'Cancel'}));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(RunInBackground).not.toHaveBeenCalled();
    expect(QuitApplication).not.toHaveBeenCalled();
    requestClose();
    await user.click(screen.getByRole('button', {name: 'Run in background'}));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(RunInBackground).toHaveBeenCalledOnce();
    expect(StopTimer).not.toHaveBeenCalled();
    expect(QuitApplication).not.toHaveBeenCalled();
  });

  it('shows quit/save failures in the close prompt and prevents duplicate submissions', async () => {
    const user = userEvent.setup();
    await renderLoaded();
    requestClose();
    let rejectQuit: (error: Error) => void = () => {throw new Error('Quit promise not initialized');};
    vi.mocked(QuitApplication).mockImplementationOnce(() => new Promise<void>((_resolve, reject) => {rejectQuit = reject;}));
    await user.click(screen.getByRole('button', {name: 'Quit'}));
    expect(screen.getByRole('button', {name: 'Quit'})).toBeDisabled();
    expect(screen.getByRole('button', {name: 'Run in background'})).toBeDisabled();
    await user.click(screen.getByRole('button', {name: 'Quit'}));
    expect(QuitApplication).toHaveBeenCalledOnce();
    await act(async () => rejectQuit(new Error('save timer before quitting: access denied')));
    expect(await screen.findByRole('alert')).toHaveTextContent('save timer before quitting: access denied');
    expect(screen.getByRole('dialog', {name: 'Close TimeManager?'})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Quit'})).toBeEnabled();
    vi.mocked(RunInBackground).mockRejectedValueOnce(new Error('data store is not ready'));
    await user.click(screen.getByRole('button', {name: 'Run in background'}));
    expect(await screen.findByRole('alert')).toHaveTextContent('data store is not ready');
    expect(screen.getByRole('dialog', {name: 'Close TimeManager?'})).toBeInTheDocument();
  });

  it('keeps an unsaved project editor when closing is cancelled and unregisters close events', async () => {
    const user = userEvent.setup();
    vi.mocked(GetState).mockResolvedValue(state());
    const rendered = render(<App />);
    await screen.findByRole('heading', {name: 'Time & calendar'});
    await user.click(screen.getByRole('button', {name: 'Projects'}));
    await user.click(screen.getByRole('button', {name: 'Copy project 100 Alpha'}));
    await user.type(screen.getByLabelText('Project name'), 'Unsaved copy');
    requestClose();
    const closeDialog = screen.getByRole('dialog', {name: 'Close TimeManager?'});
    await user.click(within(closeDialog).getByRole('button', {name: 'Cancel'}));
    expect(screen.getByRole('dialog', {name: 'Create project'})).toBeInTheDocument();
    expect(screen.getByLabelText('Project name')).toHaveValue('Unsaved copy');
    const listenerIndex = vi.mocked(EventsOn).mock.calls.findIndex(([name]) => name === 'close-requested');
    const unsubscribe = vi.mocked(EventsOn).mock.results[listenerIndex].value;
    rendered.unmount();
    expect(unsubscribe).toHaveBeenCalledOnce();
  });

  it('summarizes saved time separately per project and subproject, including archived projects', async () => {
    const user = userEvent.setup();
    const child = {...alpha, id: 'child', number: '101', name: 'Child', parentId: alpha.id};
    const booking = (id: string, projectId: string, start: string, end: string) =>
      Object.assign(new main.Booking(), {id, projectId, start, end, notes: 'Detailed note'});
    const bookings = [
      booking('a', alpha.id, '2026-10-26T09:00:00', '2026-10-26T10:00:00'),
      booking('b', alpha.id, '2026-10-26T11:00:00', '2026-10-26T12:30:00'),
      booking('c', child.id, '2026-10-26T13:00:00', '2026-10-26T13:45:00'),
      booking('d', archived.id, '2026-10-26T14:00:00', '2026-10-26T14:15:00'),
    ];
    await renderLoaded(state({projects: [alpha, child, archived], bookings}));
    fireEvent.change(screen.getByLabelText('Week of'), {target: {value: '2026-10-26'}});
    expect(screen.getAllByRole('button', {name: /Detailed note/})).toHaveLength(4);
    await user.selectOptions(screen.getByLabelText('Calendar view'), 'summary');
    const list = screen.getByRole('list', {name: 'Project totals 2026-10-26'});
    const entries = within(list).getAllByRole('listitem');
    expect(entries).toHaveLength(3);
    expect(entries[0]).toHaveTextContent('100 - Alpha');
    expect(entries[0]).toHaveTextContent('02:30:00');
    expect(entries[1]).toHaveTextContent('101 - Child');
    expect(entries[1]).toHaveTextContent('00:45:00');
    expect(entries[2]).toHaveTextContent('200 - Archived project');
    expect(entries[2]).toHaveTextContent('00:15:00');
    expect(within(list).queryByRole('button')).not.toBeInTheDocument();
    expect(screen.queryByText('Detailed note')).not.toBeInTheDocument();
    expect(SaveBooking).not.toHaveBeenCalled();
    await user.selectOptions(screen.getByLabelText('Calendar view'), 'details');
    await user.click(screen.getAllByRole('button', {name: /Detailed note/})[0]);
    expect(screen.getByRole('dialog', {name: 'Edit booking'})).toBeInTheDocument();
  });

  it('clips overnight summaries at local midnight and excludes boundary-only days', async () => {
    const user = userEvent.setup();
    const bookings = [
      Object.assign(new main.Booking(), {id: 'overnight', projectId: alpha.id, start: '2026-10-26T23:30:00', end: '2026-10-27T01:15:00', notes: ''}),
      Object.assign(new main.Booking(), {id: 'midnight', projectId: alpha.id, start: '2026-10-27T23:00:00', end: '2026-10-28T00:00:00', notes: ''}),
    ];
    await renderLoaded(state({bookings}));
    fireEvent.change(screen.getByLabelText('Week of'), {target: {value: '2026-10-26'}});
    await user.selectOptions(screen.getByLabelText('Calendar view'), 'summary');
    expect(screen.getByRole('list', {name: 'Project totals 2026-10-26'})).toHaveTextContent('00:30:00');
    expect(screen.getByRole('list', {name: 'Project totals 2026-10-27'})).toHaveTextContent('02:15:00');
    expect(screen.queryByRole('list', {name: 'Project totals 2026-10-28'})).not.toBeInTheDocument();
    expect(within(screen.getByRole('region', {name: '2026-10-28'})).getByText('No bookings')).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Next week'}));
    expect(screen.queryByRole('list', {name: /^Project totals/})).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Previous week'}));
    expect(screen.getByRole('list', {name: 'Project totals 2026-10-26'})).toHaveTextContent('00:30:00');
  });

  it('excludes the running timer from summary entries while retaining stop controls', async () => {
    const user = userEvent.setup();
    await renderLoaded(state({active: Object.assign(new main.Timer(), {projectId: alpha.id, start: new Date().toISOString()})}));
    expect(screen.getByText('Saved when stopped')).toBeInTheDocument();
    await user.selectOptions(screen.getByLabelText('Calendar view'), 'summary');
    expect(screen.queryByText('Saved when stopped')).not.toBeInTheDocument();
    expect(screen.queryByRole('list', {name: /^Project totals/})).not.toBeInTheDocument();
    expect(screen.getAllByText('No bookings')).toHaveLength(7);
    expect(screen.getByRole('button', {name: 'Stop & save'})).toBeEnabled();
    expect(StopTimer).not.toHaveBeenCalled();
    expect(ToggleTimer).not.toHaveBeenCalled();
  });

  it('can manually mark and unmark exported bookings while retaining precise timestamps', async () => {
    const user = userEvent.setup();
    const booking = Object.assign(new main.Booking(), {...existingBooking, start: '2026-10-26T09:00:00.1234567Z', end: '2026-10-26T10:00:00.7654321Z', exported: false});
    await renderLoaded(state({bookings: [booking]}));
    fireEvent.change(screen.getByLabelText('Week of'), {target: {value: '2026-10-26'}});
    await user.click(screen.getByRole('button', {name: /Existing note/}));
    await user.click(screen.getByLabelText('Exported'));
    vi.mocked(GetState).mockResolvedValue(state({bookings: [Object.assign(new main.Booking(), {...booking, exported: true})]}));
    await user.click(screen.getByRole('button', {name: 'Save booking'}));
    expect(SaveBooking).toHaveBeenLastCalledWith(expect.objectContaining({exported: true, start: booking.start, end: booking.end}));
    vi.mocked(GetState).mockResolvedValue(state({bookings: [Object.assign(new main.Booking(), {...booking, exported: true})]}));
    await user.click(screen.getByRole('button', {name: /Existing note/}));
    await user.click(screen.getByLabelText('Exported'));
    await user.click(screen.getByRole('button', {name: 'Save booking'}));
    expect(SaveBooking).toHaveBeenLastCalledWith(expect.objectContaining({exported: false}));
  });

  it('grays exported bookings and only fully exported daily project totals', async () => {
    const user = userEvent.setup();
    const bookings = [
      Object.assign(new main.Booking(), {id:'one',projectId:alpha.id,start:'2026-10-26T09:00:00',end:'2026-10-26T10:00:00',notes:'marked',exported:true}),
      Object.assign(new main.Booking(), {id:'two',projectId:alpha.id,start:'2026-10-26T10:00:00',end:'2026-10-26T11:00:00',notes:'unmarked',exported:false}),
      Object.assign(new main.Booking(), {id:'three',projectId:alpha.id,start:'2026-10-27T09:00:00',end:'2026-10-27T10:00:00',notes:'all marked',exported:true}),
    ];
    await renderLoaded(state({bookings}));
    fireEvent.change(screen.getByLabelText('Week of'), {target: {value:'2026-10-26'}});
    expect(within(screen.getByRole('region',{name:'2026-10-26'})).getByRole('button', {name:/marked\s*Exported$/})).toHaveClass('exported-booking');
    expect(screen.getByRole('button', {name:/unmarked$/})).not.toHaveClass('exported-booking');
    await user.selectOptions(screen.getByLabelText('Calendar view'),'summary');
    expect(within(screen.getByRole('list',{name:'Project totals 2026-10-26'})).getByRole('listitem')).not.toHaveClass('exported-booking');
    expect(within(screen.getByRole('list',{name:'Project totals 2026-10-27'})).getByRole('listitem')).toHaveClass('exported-booking');
  });

  it('offers opt-in marking and excluding already exported bookings', async () => {
    const user = userEvent.setup();
    await renderLoaded();
    await user.click(screen.getByRole('button',{name:'Export to Excel'}));
    const mark = screen.getByLabelText('Mark fully included bookings as exported after saving');
    const exclude = screen.getByLabelText('Exclude already exported bookings');
    expect(mark).not.toBeChecked();
    expect(exclude).not.toBeChecked();
    await user.click(mark);
    await user.click(exclude);
    await user.click(screen.getByRole('button',{name:'Choose file & export'}));
    expect(ExportBookings).toHaveBeenCalledWith(expect.any(String),expect.any(String),{markExported:true,excludeExported:true});
  });

  it('creates a booking for a selected project', async () => {
    const user = userEvent.setup();
    await renderLoaded();

    await user.click(await screen.findByRole('button', {name: /^Add booking$/}));
    expect(screen.getByRole('dialog', {name: 'Add booking'})).toBeInTheDocument();
    await selectProject(user, 'Project', /100 - Alpha/);
    await user.type(screen.getByLabelText('Notes'), 'Planning session');
    await user.click(screen.getByRole('button', {name: 'Save booking'}));

    await waitFor(() => expect(SaveBooking).toHaveBeenCalledOnce());
    const saved = vi.mocked(SaveBooking).mock.calls[0][0];
    expect(saved).toMatchObject({id: '', projectId: alpha.id, notes: 'Planning session'});
    expect(Number.isNaN(Date.parse(saved.start))).toBe(false);
    expect(Number.isNaN(Date.parse(saved.end))).toBe(false);
  });

  it('deletes an existing booking only after confirmation', async () => {
    const user = userEvent.setup();
    await renderLoaded(state({bookings: [existingBooking]}));

    await user.click(await screen.findByRole('button', {name: /Existing note/}));
    expect(screen.getByRole('dialog', {name: 'Edit booking'})).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Delete booking'}));
    const confirmation = screen.getByRole('dialog', {name: 'Delete booking?'});
    await user.click(within(confirmation).getByRole('button', {name: 'Confirm'}));

    expect(DeleteBooking).toHaveBeenCalledWith(existingBooking.id);
  });

  it('exports a date range and announces the saved file', async () => {
    const user = userEvent.setup();
    vi.mocked(ExportBookings).mockResolvedValue('C:\\exports\\TimeManager.xlsx');
    await renderLoaded();

    await user.click(await screen.findByRole('button', {name: 'Export to Excel'}));
    const dialog = screen.getByRole('dialog', {name: 'Export bookings to Excel'});
    const fromControl = within(dialog).getByLabelText('From date');
    const throughControl = within(dialog).getByLabelText('Through date (inclusive)');
    if (!(fromControl instanceof HTMLInputElement) || !(throughControl instanceof HTMLInputElement)) {
      throw new Error('Export date controls are not inputs');
    }
    const from = fromControl.value;
    const through = throughControl.value;
    await user.click(within(dialog).getByRole('button', {name: 'Choose file & export'}));

    expect(ExportBookings).toHaveBeenCalledWith(from, through, {markExported: false, excludeExported: false});
    expect(await screen.findByText('Excel export saved: C:\\exports\\TimeManager.xlsx')).toBeInTheDocument();
  });

  it('saves dark appearance and applies it to the app and native window', async () => {
    const user = userEvent.setup();
    await renderLoaded();
    vi.mocked(GetState).mockResolvedValue(state({settings: Object.assign(new tracker.Settings(), {theme: 'dark', reminders: defaultReminders()})}));
    await user.click(screen.getByRole('button', {name: 'Settings'}));
    await user.selectOptions(screen.getByLabelText('Color theme'), 'dark');
    expect(SaveSettings).toHaveBeenCalledWith(expect.objectContaining({theme: 'dark', reminders: defaultReminders()}));
    await waitFor(() => expect(document.documentElement.dataset.theme).toBe('dark'));
    expect(WindowSetDarkTheme).toHaveBeenCalled();
    expect(screen.getByRole('heading', {name: 'Project properties'})).toBeInTheDocument();
  });

  it('shows settings save errors without switching the theme', async () => {
    const user = userEvent.setup();
    vi.mocked(SaveSettings).mockRejectedValueOnce(new Error('Disk is full'));
    await renderLoaded();
    await user.click(screen.getByRole('button', {name: 'Settings'}));
    await user.selectOptions(screen.getByLabelText('Color theme'), 'dark');
    expect(await screen.findByRole('alert')).toHaveTextContent('Disk is full');
    expect(document.documentElement.dataset.theme).toBe('light');
  });

  it('creates a shared dropdown property and requires confirmation before deleting it', async () => {
    const user = userEvent.setup();
    const definition = {id: 'status', name: 'Status', type: 'dropdown', options: ['Open', 'Closed']};
    await renderLoaded();
    await user.click(screen.getByRole('button', {name: 'Settings'}));
    await user.click(screen.getByRole('button', {name: 'Add property'}));
    await user.type(screen.getByLabelText('Property name'), 'Status');
    await user.selectOptions(screen.getByLabelText('Property type'), 'dropdown');
    await user.type(screen.getByLabelText('Dropdown options (one per line)'), 'Open\nClosed');
    vi.mocked(GetState).mockResolvedValue(state({propertyDefinitions: [definition]}));
    await user.click(screen.getByRole('button', {name: 'Save property'}));
    expect(SavePropertyDefinition).toHaveBeenCalledWith({...definition, id: ''});
    const remove = await screen.findByRole('button', {name: 'Delete property Status'});
    await user.click(remove);
    expect(DeletePropertyDefinition).not.toHaveBeenCalled();
    const dialog = screen.getByRole('dialog', {name: 'Delete property?'});
    await user.click(within(dialog).getByRole('button', {name: 'Confirm'}));
    expect(DeletePropertyDefinition).toHaveBeenCalledWith('status');
  });

  it('edits all custom property types on a project and preserves explicit false', async () => {
    const user = userEvent.setup();
    const definitions = [
      {id: 'contact', name: 'Contact', type: 'text', options: []},
      {id: 'rate', name: 'Rate', type: 'number', options: []},
      {id: 'billable', name: 'Billable', type: 'checkbox', options: []},
      {id: 'status', name: 'Status', type: 'dropdown', options: ['Open', 'Closed']},
    ];
    await renderLoaded(state({propertyDefinitions: definitions}));
    await user.click(screen.getByRole('button', {name: 'Projects'}));
    await user.click(screen.getByRole('button', {name: 'Edit'}));
    await user.type(screen.getByLabelText('Contact'), 'Jane');
    await user.type(screen.getByLabelText('Rate'), '125.5');
    await user.selectOptions(screen.getByLabelText('Billable'), 'false');
    await user.selectOptions(screen.getByLabelText('Status'), 'Open');
    await user.click(screen.getByRole('button', {name: 'Save project'}));
    expect(SaveProject).toHaveBeenCalledWith(expect.objectContaining({
      properties: {contact: 'Jane', rate: '125.5', billable: 'false', status: 'Open'},
    }));
  });

  it('follows system theme changes and cleans up its listener', async () => {
    const events = new EventTarget();
    let dark = false;
    const media: MediaQueryList = {
      get matches() { return dark; },
      media: '(prefers-color-scheme: dark)',
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: events.addEventListener.bind(events),
      removeEventListener: vi.fn(events.removeEventListener.bind(events)),
      dispatchEvent: events.dispatchEvent.bind(events),
    };
    vi.mocked(window.matchMedia).mockReturnValueOnce(media);
    vi.mocked(GetState).mockResolvedValue(state({settings: Object.assign(new tracker.Settings(), {theme: 'system', reminders: defaultReminders()})}));
    const rendered = render(<App/>);
    await screen.findByRole('heading', {name: 'Time & calendar'});
    expect(document.documentElement.dataset.theme).toBe('light');
    act(() => { dark = true; events.dispatchEvent(new Event('change')); });
    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(WindowSetDarkTheme).toHaveBeenCalled();
    act(() => { dark = false; events.dispatchEvent(new Event('change')); });
    expect(document.documentElement.dataset.theme).toBe('light');
    rendered.unmount();
    expect(media.removeEventListener).toHaveBeenCalledWith('change', expect.any(Function));
  });

  it('saves separate reminder intervals and enabled states without changing appearance', async () => {
    const user = userEvent.setup();
    await renderLoaded();
    await user.click(screen.getByRole('button', {name: 'Settings'}));
    await user.clear(screen.getByLabelText('Running reminder interval (minutes)'));
    await user.type(screen.getByLabelText('Running reminder interval (minutes)'), '90');
    await user.clear(screen.getByLabelText('Idle reminder interval (minutes)'));
    await user.type(screen.getByLabelText('Idle reminder interval (minutes)'), '45');
    await user.click(screen.getByLabelText('Remind when no timer is running'));
    await user.click(screen.getByRole('button', {name: 'Save reminders'}));
    expect(SaveSettings).toHaveBeenCalledWith(expect.objectContaining({
      theme: 'light', reminders: {runningEnabled: true, runningMinutes: 90, idleEnabled: false, idleMinutes: 45},
    }));
  });

  it('previews both reminder states without saving settings', async () => {
    const user = userEvent.setup();
    await renderLoaded();
    await user.click(screen.getByRole('button', {name: 'Settings'}));
    await user.click(screen.getByRole('button', {name: 'Preview running reminder'}));
    await waitFor(() => expect(TestReminder).toHaveBeenCalledWith(true));
    await user.click(screen.getByRole('button', {name: 'Preview idle reminder'}));
    await waitFor(() => expect(TestReminder).toHaveBeenCalledWith(false));
    expect(SaveSettings).not.toHaveBeenCalled();
  });

  it('rejects zero reminder intervals in the form and surfaces bridge errors', async () => {
    const user = userEvent.setup();
    await renderLoaded();
    await user.click(screen.getByRole('button', {name: 'Settings'}));
    const interval = screen.getByLabelText('Idle reminder interval (minutes)');
    await user.clear(interval);
    await user.type(interval, '0');
    await user.click(screen.getByRole('button', {name: 'Save reminders'}));
    expect(SaveSettings).not.toHaveBeenCalled();
    await user.clear(interval);
    await user.type(interval, '30');
    vi.mocked(SaveSettings).mockRejectedValueOnce(new Error('Could not save reminders'));
    await user.click(screen.getByRole('button', {name: 'Save reminders'}));
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not save reminders');
  });
});
