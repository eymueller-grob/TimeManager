import {fireEvent, render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {useState} from 'react';
import {describe, expect, it, vi} from 'vitest';
import DatePicker from './DatePicker';

function Example({dateTime = false}: {dateTime?: boolean}) {
    const [value, setValue] = useState(dateTime ? '2027-01-01T10:20:30' : '2027-01-01');
    return <DatePicker label="Date" value={value} onChange={setValue} dateTime={dateTime}/>;
}

async function openCalendar(user: ReturnType<typeof userEvent.setup>) {
    await user.click(screen.getByLabelText('Date'));
}

describe('ISO week date picker', () => {
    it('edits separate date and time fields without losing the other part', async () => {
        const user = userEvent.setup();
        render(<Example dateTime/>);
        expect(screen.getByLabelText('Date')).toHaveValue('2027-01-01');
        expect(screen.getByLabelText('Time for Date')).toHaveValue('10:20:30');
        await user.clear(screen.getByLabelText('Date'));
        await user.type(screen.getByLabelText('Date'), '2027-02-03');
        expect(screen.getByLabelText('Time for Date')).toHaveValue('10:20:30');
        expect(screen.getByLabelText('Date')).toBeValid();
        fireEvent.change(screen.getByLabelText('Time for Date'), {target: {value: ''}});
        expect(screen.getByLabelText('Date')).toHaveValue('2027-02-03');
        expect(screen.getByLabelText('Date')).toBeInvalid();
        fireEvent.change(screen.getByLabelText('Time for Date'), {target: {value: '14:45:50'}});
        expect(screen.getByLabelText('Date')).toBeValid();
        expect(screen.queryByRole('button', {name: /Open calendar/})).not.toBeInTheDocument();
        await openCalendar(user);
        await user.click(screen.getByRole('button', {name: '2027-02-04'}));
        expect(screen.getByLabelText('Date')).toHaveValue('2027-02-04');
        expect(screen.getByLabelText('Time for Date')).toHaveValue('14:45:50');
    });

    it('shows ISO week-year rows across New Year and selects a date', async () => {
        const user = userEvent.setup();
        render(<Example/>);
        await openCalendar(user);
        const calendar = screen.getByRole('region', {name: 'Date calendar'});
        expect(within(calendar).getByText('CW')).toBeInTheDocument();
        expect(within(calendar).getByText('CW53')).toBeInTheDocument();
        expect(within(calendar).getByText('CW01')).toBeInTheDocument();
        const selectedDay = screen.getByRole('button', {name: '2027-01-01'});
        expect(selectedDay).toHaveClass('rounded-lg');
        expect(selectedDay.parentElement).toHaveClass('rounded-lg');
        await user.click(screen.getByRole('button', {name: '2027-01-04'}));
        expect(screen.getByLabelText('Date')).toHaveValue('2027-01-04');
        expect(screen.queryByRole('region')).not.toBeInTheDocument();
        expect(screen.getByText('CW01')).toBeInTheDocument();
    });

    it('preserves time, supports time editing, and Escape dismisses only the picker', async () => {
        const user = userEvent.setup();
        const escape = vi.fn();
        render(<div onKeyDown={escape}><Example dateTime/></div>);
        await openCalendar(user);
        await user.click(screen.getByRole('button', {name: '2027-01-04'}));
        expect(screen.getByLabelText('Date')).toHaveValue('2027-01-04');
        expect(screen.getByLabelText('Time for Date')).toHaveValue('10:20:30');
        fireEvent.change(screen.getByLabelText('Time for Date'), {target: {value: '12:34:56'}});
        expect(screen.getByLabelText('Date')).toHaveValue('2027-01-04');
        expect(screen.getByLabelText('Time for Date')).toHaveValue('12:34:56');
        await openCalendar(user);
        escape.mockClear();
        await user.keyboard('{Escape}');
        expect(screen.queryByRole('region')).not.toBeInTheDocument();
        expect(escape).not.toHaveBeenCalled();
        expect(screen.getByLabelText('Date')).toHaveFocus();
    });

    it('handles keyboard date and month navigation', async () => {
        const user = userEvent.setup();
        render(<Example/>);
        await openCalendar(user);
        await user.keyboard('{ArrowDown}');
        expect(screen.getByRole('button', {name: '2027-01-01'})).toHaveFocus();
        await user.keyboard('{ArrowDown}');
        await vi.waitFor(() => expect(screen.getByRole('button', {name: '2027-01-08'})).toHaveFocus());
        await user.keyboard('{PageDown}');
        await vi.waitFor(() => expect(screen.getByRole('button', {name: '2027-02-08'})).toHaveFocus());
        await user.keyboard('{Enter}');
        expect(screen.getByLabelText('Date')).toHaveValue('2027-02-08');
    });

    it('enforces date bounds and rejects invalid typed dates', async () => {
        const user = userEvent.setup();
        const change = vi.fn();
        const {rerender} = render(<DatePicker label="Date" value="2027-01-04" min="2027-01-04" max="2027-01-08" onChange={change}/>);
        await openCalendar(user);
        expect(screen.getByRole('button', {name: '2027-01-03'})).toBeDisabled();
        await user.click(screen.getByRole('button', {name: '2027-01-03'}));
        expect(change).not.toHaveBeenCalled();
        await user.click(screen.getByRole('button', {name: '2027-01-05'}));
        expect(change).toHaveBeenCalledWith('2027-01-05');
        rerender(<DatePicker label="Date" value="2027-02-30" onChange={change}/>);
        expect(screen.getByLabelText('Date')).toBeInvalid();
    });

    it('respects disabled fieldsets and dismisses on outside interaction', async () => {
        const user = userEvent.setup();
        const {rerender} = render(<fieldset disabled><Example/></fieldset>);
        expect(screen.getByLabelText('Date')).toBeDisabled();
        rerender(<><Example/><button>Outside</button></>);
        await openCalendar(user);
        await user.click(screen.getByRole('button', {name: 'Outside'}));
        expect(screen.queryByRole('region')).not.toBeInTheDocument();
    });

    it('handles leap days and date-time bounds with optional seconds', async () => {
        const user = userEvent.setup();
        const change = vi.fn();
        const {rerender} = render(<DatePicker label="Date" value="2024-02-29" onChange={change}/>);
        await openCalendar(user);
        expect(screen.getByRole('button', {name: '2024-02-29'}).parentElement).toHaveAttribute('aria-selected', 'true');
        await user.click(screen.getByRole('button', {name: 'Next month for Date'}));
        expect(screen.getByRole('button', {name: '2024-03-01'})).toHaveAttribute('tabindex', '0');
        rerender(<DatePicker label="Date" value="2024-02-29T12:34" dateTime min="2024-02-29T12:34:00" max="2024-02-29T12:34:00" onChange={change}/>);
        expect(screen.getByLabelText('Date')).toBeValid();
        rerender(<DatePicker label="Date" value="2024-02-29T12:34:01" dateTime max="2024-02-29T12:34:00" onChange={change}/>);
        expect(screen.getByLabelText('Date')).toBeInvalid();
        expect(screen.getByRole('alert')).toHaveTextContent('at or before');
    });
});
