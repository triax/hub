// __tests__/index.test.jsx
/**
 * @jest-environment jsdom
 */

import * as r from 'next/router';
(r as any).useRouter = jest.fn();
(r as any).useRouter.mockImplementation(() => ({ route: '/' }));
import '@testing-library/jest-dom/extend-expect';
import { Fetch } from 'jestil';

import React from 'react'
import { render, screen } from '@testing-library/react'
import Top from "../client/pages/index"
import Member from "../client/models/Member";

describe('Top', () => {
  it('renders a heading', () => {
    Fetch.replies([{}, {}]);
    render(<Top myself={Member.placeholder()} />)

    const heading = screen.getByRole('heading')

    expect(heading).toBeInTheDocument()
  })
})
