import React from 'react';
import { Link, useParams } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import { InteractiveButton } from '../components/InteractiveButton';

const ProductDetailPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();

  // TODO: Fetch detailed product data based on ID
  // This will include: Ad analytics, Amazon data, competitor analysis, sourcing info

  return (
    <div className="min-h-screen bg-black text-white p-6">
      <div className="max-w-7xl mx-auto">
        <Link to="/dashboard">
          <InteractiveButton variant="ghost" className="mb-8">
            <ArrowLeft size={20} className="mr-2" />
            Back to Dashboard
          </InteractiveButton>
        </Link>

        <div className="text-center py-32">
          <div className="inline-block bg-zinc-900/50 border-2 border-white/10 rounded-3xl p-12">
            <h1 className="font-display text-4xl font-black text-white mb-4">
              Product Detail View
            </h1>
            <p className="text-zinc-400 font-mono mb-4">Product ID: {id}</p>
            <p className="text-zinc-400 font-mono mb-8">
              Comprehensive validation data, metrics, and sourcing info coming soon.
            </p>
            <Link to="/dashboard">
              <InteractiveButton className="px-8 py-4">
                Return to Dashboard
              </InteractiveButton>
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
};

export default ProductDetailPage;
