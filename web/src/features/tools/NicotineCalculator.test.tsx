import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { afterEach, it, expect } from 'vitest';
import { NicotineCalculator } from './NicotineCalculator';
afterEach(cleanup);
it('invalidates a computed result as soon as any input changes', () => {
  render(<NicotineCalculator />);
  fireEvent.change(screen.getByLabelText('Flaschenmenge'), { target: { value: '120' } });
  fireEvent.change(screen.getByLabelText('Aroma'), { target: { value: '10' } });
  fireEvent.change(screen.getByLabelText('Nikotinshots'), { target: { value: '20' } });
  fireEvent.change(screen.getByLabelText('Stärke der Shots'), { target: { value: '20' } });
  fireEvent.click(screen.getByRole('button', { name: 'Berechnen' }));
  expect(screen.getByText('90,000 ml')).toBeInTheDocument();
  fireEvent.change(screen.getByLabelText('Stärke der Shots'), { target: { value: '18' } });
  expect(screen.queryByText('90,000 ml')).not.toBeInTheDocument();
  expect(screen.queryByText(/mg\/l/)).not.toBeInTheDocument();
});
