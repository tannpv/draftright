interface StatCardProps {
  icon: string;
  label: string;
  value: string | number;
  color?: 'blue' | 'green' | 'purple' | 'orange';
}

const colorMap = {
  blue: 'bg-blue-100 text-blue-600',
  green: 'bg-green-100 text-green-600',
  purple: 'bg-purple-100 text-purple-600',
  orange: 'bg-orange-100 text-orange-600',
};

export default function StatCard({ icon, label, value, color = 'blue' }: StatCardProps) {
  return (
    <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6 flex items-center gap-4">
      <div className={`flex items-center justify-center w-12 h-12 rounded-lg text-2xl ${colorMap[color]}`}>
        {icon}
      </div>
      <div>
        <p className="text-sm text-gray-500 font-medium">{label}</p>
        <p className="text-2xl font-bold text-gray-900 mt-0.5">{value}</p>
      </div>
    </div>
  );
}
